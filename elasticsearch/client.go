package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/hasura/ndc-sdk-go/connector"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Client struct {
	// client is accessed by many concurrent goroutines (one per query) and
	// replaced on reauthentication. atomic.Pointer gives lock-free reads while
	// keeping writes safe.
	client atomic.Pointer[elasticsearch.Client]

	// reauthMu serializes credential refreshes. When multiple goroutines receive
	// HTTP 401 simultaneously, only the first one to acquire this lock actually
	// calls Reauthenticate; the rest wait, then detect that e.client has already
	// been replaced and skip straight to the retry.
	reauthMu sync.Mutex
}

func (e *Client) getClient() *elasticsearch.Client {
	return e.client.Load()
}

func (e *Client) setClient(c *elasticsearch.Client) {
	e.client.Store(c)
}

// NewClient creates a new client with configuration from cfg.
func NewClient(ctx context.Context) (*Client, error) {
	client := &Client{}
	err := client.Authenticate(ctx)
	if err != nil {
		return nil, fmt.Errorf("error authenticating with elasticsearch: %s", err)
	}
	return client, nil
}

func (e *Client) Authenticate(ctx context.Context) error {
	logger := connector.GetLogger(ctx)
	ctx, span := otel.Tracer("es_client").Start(ctx, "authenticate_elasticsearch", trace.WithAttributes(
		attribute.String("internal.visibility", "user"), // this attr makes the span visible in the hasura console
	))
	defer span.End()

	// we'll set all auth related errors as internal errors
	// so that we don't expose any sensitive information in the API response.
	// actual errors are recorded in the span
	esConfig, err := e.accquireAuthConfig(ctx, false)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get AuthConfig")
		logger.DebugContext(ctx, "failed to get AuthConfig", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(fmt.Errorf("failed to get AuthConfig: %w", err))
		return errors.New("internal error")
	}
	esClient, err := elasticsearch.NewClient(*esConfig)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create elasticsearch client")
		logger.DebugContext(ctx, "failed to create elasticsearch client", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(fmt.Errorf("failed to create elasticsearch client: %w", err))
		return errors.New("internal error")
	}
	// Validate before publishing: ping first, then setClient. This ensures
	// getClient() never vends an unvalidated client to concurrent goroutines,
	// which also prevents a spurious second reauth when another goroutine
	// snapshots a freshly-stored-but-not-yet-pinged client and mistakes it for
	// the stale one that caused the original 401.
	err = pingClient(esClient)
	if err == nil {
		e.setClient(esClient)
		logger.InfoContext(ctx, "authentication successful")
		return nil
	}

	logger.ErrorContext(ctx, "failed to ping elasticsearch")
	logger.DebugContext(ctx, "elasticsearch ping error", "error", err)

	// if the ping fails, try to authenticate again with force refreshing the credentials
	esConfig, err = e.accquireAuthConfig(ctx, true)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get AuthConfig")
		logger.DebugContext(ctx, "failed to get AuthConfig", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(fmt.Errorf("failed to get AuthConfig: %w", err))
		return errors.New("internal error")
	}
	esClient, err = elasticsearch.NewClient(*esConfig)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create elasticsearch client")
		logger.DebugContext(ctx, "failed to create elasticsearch client", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(fmt.Errorf("failed to create elasticsearch client: %w", err))
		return errors.New("internal error")
	}
	err = pingClient(esClient)
	if err != nil {
		logger.ErrorContext(ctx, "failed to ping elasticsearch")
		logger.DebugContext(ctx, "elasticsearch ping error", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(fmt.Errorf("failed to ping elasticsearch: %w", err))
		return errors.New("internal error")
	}
	e.setClient(esClient)
	logger.InfoContext(ctx, "authentication successful")
	return nil
}

func (e *Client) Reauthenticate(ctx context.Context) error {
	logger := connector.GetLogger(ctx)
	logger.InfoContext(ctx, "reauthenticating after 401")
	ctx, span := otel.Tracer("es_client").Start(ctx, "reauthenticate_elasticsearch", trace.WithAttributes(
		attribute.String("internal.visibility", "user"),
		attribute.String("trigger", "http_401"),
	))
	defer span.End()
	if err := e.Authenticate(ctx); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (e *Client) accquireAuthConfig(ctx context.Context, forceRefresh bool) (*elasticsearch.Config, error) {
	logger := connector.GetLogger(ctx)
	if shouldUseCredentialsProvider() {
		logger.DebugContext(ctx, "using credentials provider")
		esConfig, err := getConfigFromCredentialsProvider(ctx, forceRefresh)
		if err != nil {
			return nil, err
		}
		return esConfig, nil
	} else {
		esConfig, err := getConfigFromEnv(ctx)
		if err != nil {
			return nil, err
		}
		return esConfig, nil
	}
}

// Ping returns whether the Elasticsearch cluster is running.
func (e *Client) Ping() error {
	return pingClient(e.getClient())
}

// pingClient pings c directly, without touching the Client wrapper.
// Authenticate calls this with the freshly-created *elasticsearch.Client so
// that it pings the specific instance it just built rather than whatever
// e.client happens to hold at call time.
func pingClient(c *elasticsearch.Client) error {
	res, err := c.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping elasticsearch: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("failed to ping elasticsearch: %s", res.String())
	}
	return nil
}

// Search performs a search operation in elastic search.
func (e *Client) Search(ctx context.Context, index string, body map[string]interface{}) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}

	search := esapi.Search(func(o ...func(*esapi.SearchRequest)) (*esapi.Response, error) {
		return e.search(ctx, o...)
	})

	res, err := search(
		search.WithContext(ctx),
		search.WithIndex(index),
		search.WithBody(&buf),
	)
	if err != nil {
		return nil, err
	}

	result, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	return result.(map[string]interface{}), nil
}

// search is a helper function to perform a search operation in elastic search.
func (e *Client) search(ctx context.Context, o ...func(*esapi.SearchRequest)) (*esapi.Response, error) {
	logger := connector.GetLogger(ctx)

	req := &esapi.SearchRequest{}

	for _, opt := range o {
		opt(req)
	}

	// req.Body is an io.Reader that is fully drained by req.Do. If we retry the
	// request after a 401 (see below) by calling req.Do again on the same req,
	// the already-drained reader sends an empty body. Elasticsearch treats an
	// empty _search body as a match_all query and returns unfiltered results.
	// To make the request safely repeatable, capture the encoded body once and
	// rebuild a fresh reader before every attempt.
	body, err := drainBody(req.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading search request body: %w", err)
	}
	index := strings.Join(req.Index, ",")

	// Snapshot the client pointer used for this attempt. On a 401 we compare
	// this snapshot against the current pointer to decide whether another
	// goroutine already reauthenticated while we were waiting for reauthMu.
	firstClient := e.getClient()

	req.Body = bytes.NewReader(body)
	logger.InfoContext(ctx, "Query", "index", index, "body_bytes", req.Body.(*bytes.Reader).Len())
	// Check the transport error before touching res: on a transport-level
	// failure res is nil and res.IsError() would panic.
	res, err := req.Do(ctx, firstClient)
	if err != nil {
		return nil, fmt.Errorf("error while querying: %s", err)
	}

	if res.IsError() {
		if res.StatusCode != 401 {
			return nil, fmt.Errorf("error while querying: %s", res.String())
		}

		span := trace.SpanFromContext(ctx)
		span.AddEvent("http_401_received", trace.WithAttributes(attribute.String("index", index)))
		logger.DebugContext(ctx, "401 received, acquiring reauth lock", "index", index)

		// Serialize reauthentication: only one goroutine refreshes credentials;
		// others wait here and then detect that e.client was already replaced.
		e.reauthMu.Lock()
		if e.getClient() == firstClient {
			// Client hasn't been replaced yet — this goroutine does the reauth.
			span.AddEvent("reauth_started")
			if err = e.Reauthenticate(ctx); err != nil {
				e.reauthMu.Unlock()
				span.SetStatus(codes.Error, err.Error())
				return nil, fmt.Errorf("error: %s", err)
			}
			span.AddEvent("reauth_completed")
		} else {
			// Another concurrent goroutine already rotated the client while we
			// were waiting for reauthMu — skip the refresh and go straight to
			// the retry with the already-replaced client.
			span.AddEvent("reauth_skipped", trace.WithAttributes(
				attribute.String("reason", "completed_by_concurrent_goroutine"),
			))
			logger.DebugContext(ctx, "reauth already completed by concurrent goroutine, retrying", "index", index)
		}
		retryClient := e.getClient()
		e.reauthMu.Unlock()

		req.Body = bytes.NewReader(body)
		span.AddEvent("retrying_after_reauth", trace.WithAttributes(attribute.String("index", index)))
		logger.InfoContext(ctx, "Retry Query", "index", index, "body_bytes", req.Body.(*bytes.Reader).Len())
		_, retrySpan := otel.Tracer("es_client").Start(ctx, "retry_query")
		retrySpan.SetAttributes(
			attribute.String("db.statement", string(body)),
			attribute.String("db.elasticsearch.path_parts.index", index),
			attribute.String("db.operation", "search"),
			attribute.String("http.request.method", "POST"),
			attribute.String("db.system", "elasticsearch"),
		)
		res, err = req.Do(ctx, retryClient)
		if err != nil {
			retrySpan.SetStatus(codes.Error, err.Error())
			retrySpan.End()
			return nil, fmt.Errorf("error: %s", err)
		}
		retrySpan.End()
	}
	return res, err
}

// drainBody reads an io.Reader fully and returns its bytes, handling a nil
// reader (no body) gracefully.
func drainBody(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	return io.ReadAll(r)
}

// Explain performs a search with explain operation in elastic search.
//
// Since the Explain API requires document ID, we can't use it.
// Explain API: https://www.elastic.co/guide/en/elasticsearch/reference/current/search-explain.html
//
// We instead use the Profile API to implement query explain functionality.
// To use the Profile API, we need to add `profile=true` to the query.
// Profile API: https://www.elastic.co/guide/en/elasticsearch/reference/current/search-profile.html
func (e *Client) ExplainSearch(ctx context.Context, index string, query map[string]interface{}) (map[string]interface{}, error) {
	// Add `profile=true` to the query to get profiling query information
	query["profile"] = true

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, err
	}

	// `profile=true` is added to the query in the Buffer
	// We can safely remove it from the query map, so that the original var remains unchanged
	delete(query, "profile")

	search := esapi.Search(func(o ...func(*esapi.SearchRequest)) (*esapi.Response, error) {
		return e.search(ctx, o...)
	})

	res, err := search(
		search.WithContext(ctx),
		search.WithIndex(index),
		search.WithBody(&buf),
		search.WithExplain(true), // set explain to true
	)
	if err != nil {
		return nil, err
	}

	result, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	return result.(map[string]interface{}), nil
}

// GetIndices Returns comma seperated list of indices that matches the ELASTICSEARCH_INDEX_PATTERN env character.
func (e *Client) GetIndices(ctx context.Context) ([]string, error) {
	// Create a request to retrieve indices matching the regex pattern
	defaultIndex := "*,-.*"

	indexPattern := os.Getenv("ELASTICSEARCH_INDEX_PATTERN")
	if indexPattern == "" {
		indexPattern = defaultIndex
	}

	req := esapi.CatIndicesRequest{
		Index:  []string{indexPattern},
		Format: "json",
	}

	// Perform the request
	res, err := req.Do(context.Background(), e.getClient())
	if err != nil {
		return nil, fmt.Errorf("error getting indices: %s", err)
	}

	indices, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0)

	// Print the indices matching the regex pattern.
	for _, index := range indices.([]interface{}) {
		indexName, ok := index.(map[string]interface{})["index"]
		if !ok {
			continue
		}
		result = append(result, indexName.(string))
	}

	return result, nil
}

// GetAliases Returns aliases for all indices.
func (e *Client) GetAliases(ctx context.Context) (aliasToIndexMap map[string]string, err error) {
	// Get aliases for all indices
	req := esapi.CatAliasesRequest{
		Name:   []string{},
		Format: "json",
	}

	// Perform the request
	res, err := req.Do(context.Background(), e.getClient())
	if err != nil {
		return nil, fmt.Errorf("error getting aliases: %s", err)
	}

	result, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	resultJson, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("error marshalling alias result to JSON: %s", err)
	}

	// Parse alias result to get alias to index mapping
	unmarshalledJsonArray := make([]map[string]string, 0)
	err = json.Unmarshal(resultJson, &unmarshalledJsonArray)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling alias result: %s", err)
	}

	aliasToIndexMap = make(map[string]string)

	for _, jsonObj := range unmarshalledJsonArray {
		aliasToIndexMap[jsonObj["alias"]] = jsonObj["index"]
	}

	return aliasToIndexMap, nil
}

// AddAliasesToMappings adds aliases to mappings
// Each alias is added as a separate index
// The mappings of original index are copied to alias index
func (e *Client) AddAliasesToMappings(ctx context.Context, aliasToIndexMap map[string]string, mappings map[string]interface{}) {
	// Add aliases to mappings
	for alias, index := range aliasToIndexMap {
		if mappings[index] == nil {
			// index not present in mappings, don't add alias
			continue
		}
		mappings[alias] = mappings[index]
	}
}

// GetMappings Returns mappings for comma seperated list of indices.
func (e *Client) GetMappings(ctx context.Context, indices []string) (mappings map[string]interface{}, err error) {
	req := esapi.IndicesGetMappingRequest{
		Index: indices,
	}

	// Perform the request
	res, err := req.Do(context.Background(), e.getClient())
	if err != nil {
		return nil, fmt.Errorf("error getting mappings: %s", err)
	}

	result, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	mappings, ok := result.(map[string]interface{})
	if !ok {
		return nil, errors.New("failed to convert mappings to map[string]interface{}")
	}

	return mappings, nil

}

// parseResponse parses the response from esapi and handles errors.
func parseResponse(ctx context.Context, res *esapi.Response) (interface{}, error) {
	logger := connector.GetLogger(ctx)
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return nil, fmt.Errorf("error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			root_cause, _ := e["error"].(map[string]interface{})["root_cause"].([]interface{})[0].(map[string]interface{})
			errMsg := fmt.Sprintf("[%s] %s: %s",
				res.Status(),
				root_cause["type"],
				root_cause["reason"],
			)
			logger.DebugContext(ctx, "Response Details", "response", e)
			return nil, errors.New(errMsg)
		}
	}

	var result interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error parsing the response body: %s", err)
	}

	return result, nil
}

// GetInfo retrieves information about Elasticsearch.
func (e *Client) GetInfo(ctx context.Context) (interface{}, error) {
	req := esapi.InfoRequest{}
	res, err := req.Do(ctx, e.getClient())
	if err != nil {
		return nil, fmt.Errorf("error getting elasticsearch information: %s", err)
	}

	result, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	return result, nil
}
