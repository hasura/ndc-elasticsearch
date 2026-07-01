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

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/hasura/ndc-sdk-go/connector"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Client struct {
	client *elasticsearch.Client
}

// NewClient creates a new client with configuration from cfg.
func NewClient(ctx context.Context) (*Client, error) {
	client := &Client{}
	err := client.Authenticate(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not authenticate with elasticsearch: %w", err)
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
		logger.ErrorContext(ctx, "failed to get AuthConfig", "category", "es_auth_or_connectivity")
		logger.DebugContext(ctx, "failed to get AuthConfig", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(fmt.Errorf("failed to get AuthConfig: %w", err))
		return errors.New("elasticsearch authentication configuration is invalid or missing: set ELASTICSEARCH_URL and credentials (ELASTICSEARCH_USERNAME/PASSWORD or ELASTICSEARCH_API_KEY)")
	}
	esClient, err := elasticsearch.NewClient(*esConfig)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create elasticsearch client", "category", "es_auth_or_connectivity")
		logger.DebugContext(ctx, "failed to create elasticsearch client", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(fmt.Errorf("failed to create elasticsearch client: %w", err))
		return errors.New("failed to initialize elasticsearch client from the provided configuration; check ELASTICSEARCH_URL/credentials/CA cert")
	}
	e.client = esClient
	// Ping the client to check if the connection is successful
	err = e.Ping(ctx)
	if err == nil {
		// authenticated successfully
		return nil
	}

	logger.ErrorContext(ctx, "failed to ping elasticsearch")
	logger.DebugContext(ctx, "elasticsearch ping error", "error", err)

	// if the ping fails, try to authenticate again with force refreshing the credentials
	esConfig, err = e.accquireAuthConfig(ctx, true)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get AuthConfig", "category", "es_auth_or_connectivity")
		logger.DebugContext(ctx, "failed to get AuthConfig", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(fmt.Errorf("failed to get AuthConfig: %w", err))
		return errors.New("elasticsearch authentication configuration is invalid or missing: set ELASTICSEARCH_URL and credentials (ELASTICSEARCH_USERNAME/PASSWORD or ELASTICSEARCH_API_KEY)")
	}
	esClient, err = elasticsearch.NewClient(*esConfig)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create elasticsearch client", "category", "es_auth_or_connectivity")
		logger.DebugContext(ctx, "failed to create elasticsearch client", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(fmt.Errorf("failed to create elasticsearch client: %w", err))
		return errors.New("failed to initialize elasticsearch client from the provided configuration; check ELASTICSEARCH_URL/credentials/CA cert")
	}
	e.client = esClient
	// Ping the client to check if the connection is successful
	err = e.Ping(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "failed to ping elasticsearch", "category", "es_auth_or_connectivity")
		logger.DebugContext(ctx, "elasticsearch ping error", "error", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(fmt.Errorf("failed to ping elasticsearch: %w", err))
		return errors.New("elasticsearch authentication failed: credentials were rejected or the cluster is unreachable after re-authentication (HTTP 401 / connection). Verify credentials and that they are not expired/rotated")
	}

	return nil
}

func (e *Client) Reauthenticate(ctx context.Context) error {
	return e.Authenticate(ctx)
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
func (e *Client) Ping(ctx context.Context) error {
	logger := connector.GetLogger(ctx)
	res, err := e.client.Ping()
	if err != nil {
		return fmt.Errorf("cannot connect to elasticsearch (network/TLS error); check ELASTICSEARCH_URL and connectivity: %w", err)
	}
	if res.IsError() {
		// Keep the raw cluster response at DEBUG only; never surface it to consumers.
		logger.DebugContext(ctx, "elasticsearch ping error response", "status", res.StatusCode, "body", res.String())
		return fmt.Errorf("elasticsearch health check failed: cluster returned HTTP %d (%s); verify ELASTICSEARCH_URL and that the cluster is reachable", res.StatusCode, res.Status())
	}

	defer res.Body.Close()

	return nil
}

// Search performs a search operation in elastic search.
func (e *Client) Search(ctx context.Context, index string, body map[string]interface{}) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("failed to encode the search query (internal): %w", err)
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

	// First attempt.
	req.Body = bytes.NewReader(body)
	logger.DebugContext(ctx, "Query", "index", index, "body_bytes", req.Body.(*bytes.Reader).Len())
	res, err := req.Do(ctx, e.client)
	// Check the transport error before touching res: on a transport-level
	// failure res is nil and res.IsError() would panic.
	if err != nil {
		return nil, fmt.Errorf("elasticsearch request failed before a response (network/TLS); check connectivity to the cluster: %w", err)
	}

	if res.IsError() {
		if res.StatusCode == 401 {
			// Unauthorized error, reauthenticate and retry.
			if err = e.Reauthenticate(ctx); err != nil {
				return nil, fmt.Errorf("re-authentication after an HTTP 401 failed; credentials may be expired/rotated or invalid: %w", err)
			}
			// Rebuild the body so the retried request carries the same query
			// instead of the already-drained (empty) reader. The retried reader
			// holds exactly these bytes, so logging string(body) reflects what is
			// actually sent over the wire.
			req.Body = bytes.NewReader(body)
			logger.DebugContext(ctx, "Retry Query", "index", index, "body_bytes", req.Body.(*bytes.Reader).Len())
			res, err = req.Do(ctx, e.client)
			if err != nil {
				return nil, fmt.Errorf("elasticsearch request failed on the post-401 retry (network/TLS): %w", err)
			}
		} else {
			// Keep the raw cluster response body at DEBUG only; never surface it to consumers.
			logger.DebugContext(ctx, "es search error response", "status", res.StatusCode, "index", index, "body", res.String())
			return nil, fmt.Errorf("elasticsearch returned an error for the search (HTTP %d %s); see debug logs for details", res.StatusCode, res.Status())
		}
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
		return nil, fmt.Errorf("failed to encode the search query (internal): %w", err)
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
	res, err := req.Do(context.Background(), e.client)
	if err != nil {
		return nil, fmt.Errorf("failed to list elasticsearch indices (check connectivity/permissions): %w", err)
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
	res, err := req.Do(context.Background(), e.client)
	if err != nil {
		return nil, fmt.Errorf("failed to list elasticsearch aliases (check connectivity/permissions): %w", err)
	}

	result, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	resultJson, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to process the elasticsearch alias response (internal): %w", err)
	}

	// Parse alias result to get alias to index mapping
	unmarshalledJsonArray := make([]map[string]string, 0)
	err = json.Unmarshal(resultJson, &unmarshalledJsonArray)
	if err != nil {
		return nil, fmt.Errorf("failed to process the elasticsearch alias response (internal): %w", err)
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
	res, err := req.Do(context.Background(), e.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get elasticsearch index mappings (check connectivity/permissions): %w", err)
	}

	result, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	mappings, ok := result.(map[string]interface{})
	if !ok {
		return nil, errors.New("failed to parse the elasticsearch mappings response (unexpected format)")
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
			// The error response body itself failed to decode; keep raw err at DEBUG.
			logger.DebugContext(ctx, "failed to parse elasticsearch error response body", "status", res.StatusCode, "error", err)
			return nil, fmt.Errorf("failed to parse the elasticsearch response (HTTP %s); see debug logs: %w", res.Status(), err)
		}
		// Keep the full ES error object (which may include root_cause.reason, field
		// values, and other potentially sensitive detail) at DEBUG only. Never
		// surface it to the consumer.
		logger.DebugContext(ctx, "Response Details", "response", e)

		// Extract the error category (root_cause.type) defensively; it is a safe,
		// non-sensitive classifier unlike root_cause.reason.
		causeType := extractRootCauseType(e)
		guidance := guidanceForRootCauseType(causeType)
		if guidance != "" {
			return nil, fmt.Errorf("elasticsearch rejected the request: %s - %s (HTTP %s); see debug logs for details", causeType, guidance, res.Status())
		}
		return nil, fmt.Errorf("elasticsearch rejected the request: %s (HTTP %s); see debug logs for details", causeType, res.Status())
	}

	var result interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		logger.DebugContext(ctx, "failed to parse elasticsearch response body", "status", res.StatusCode, "error", err)
		return nil, fmt.Errorf("failed to parse the elasticsearch response (HTTP %s); see debug logs: %w", res.Status(), err)
	}

	return result, nil
}

// extractRootCauseType safely pulls error.root_cause[0].type out of a decoded
// Elasticsearch error response. It returns "unknown" when the structure is not
// present, so a malformed error body never panics. Only the non-sensitive type
// classifier is returned; root_cause.reason is intentionally never extracted.
func extractRootCauseType(e map[string]interface{}) string {
	errObj, ok := e["error"].(map[string]interface{})
	if !ok {
		return "unknown"
	}
	rootCauses, ok := errObj["root_cause"].([]interface{})
	if !ok || len(rootCauses) == 0 {
		// Fall back to the top-level error.type when root_cause is absent.
		if t, ok := errObj["type"].(string); ok && t != "" {
			return t
		}
		return "unknown"
	}
	rootCause, ok := rootCauses[0].(map[string]interface{})
	if !ok {
		return "unknown"
	}
	if t, ok := rootCause["type"].(string); ok && t != "" {
		return t
	}
	return "unknown"
}

// guidanceForRootCauseType maps common Elasticsearch root_cause types to fixed,
// non-sensitive remediation guidance. Returns "" when no specific guidance is
// known for the type.
func guidanceForRootCauseType(causeType string) string {
	switch causeType {
	case "security_exception":
		return "authorization failed"
	case "index_not_found_exception":
		return "index does not exist"
	case "parsing_exception", "x_content_parse_exception":
		return "the generated query was invalid"
	default:
		return ""
	}
}

// GetInfo retrieves information about Elasticsearch.
func (e *Client) GetInfo(ctx context.Context) (interface{}, error) {
	req := esapi.InfoRequest{}
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get elasticsearch cluster information (check connectivity/permissions): %w", err)
	}

	result, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	return result, nil
}
