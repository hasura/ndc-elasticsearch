package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

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
		return nil, fmt.Errorf("error authenticating with elasticsearch: %s", err)
	}
	return client, nil
}

func (e *Client) Authenticate(ctx context.Context) error {
	fmt.Println("Authenticating with elasticsearch")
	ctx, span := otel.Tracer("es_client").Start(ctx, "authenticate_elasticsearch", trace.WithAttributes(
		attribute.String("internal.visibility", "user"), // this attr makes the span visible in the hasura console
	))
	defer span.End()

	// we'll set all auth related errors as internal errors
	// so that we don't expose any sensitive information in the API response.
	// actual errors are recorded in the span
	esConfig, err := e.accquireAuthConfig(ctx, false)
	if err != nil {
		fmt.Printf("Error getting config from credentials provider: %v\n", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return errors.New("internal error")
	}
	esClient, err := elasticsearch.NewClient(*esConfig)
	if err != nil {
		fmt.Printf("Error creating elasticsearch client: %v\n", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return errors.New("internal error")
	}
	e.client = esClient
	// Ping the client to check if the connection is successful
	err = e.Ping()
	if err == nil {
		// authenticated successfully
		return nil
	}

	fmt.Printf("Error pinging elasticsearch: %v\n", err)

	// if the ping fails, try to authenticate again with force refreshing the credentials
	esConfig, err = e.accquireAuthConfig(ctx, true)
	if err != nil {
		fmt.Printf("Error getting config from credentials provider: %v\n", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return errors.New("internal error")
	}
	esClient, err = elasticsearch.NewClient(*esConfig)
	if err != nil {
		fmt.Printf("Error creating elasticsearch client: %v\n", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return errors.New("internal error")
	}
	e.client = esClient
	// Ping the client to check if the connection is successful
	err = e.Ping()
	if err != nil {
		fmt.Printf("Error pinging elasticsearch: %v\n", err)
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return errors.New("internal error")
	}

	return nil
}

func (e *Client) Reauthenticate(ctx context.Context) error {
	return e.Authenticate(ctx)
}

func (e *Client) accquireAuthConfig(ctx context.Context, forceRefresh bool) (*elasticsearch.Config, error) {
	if shouldUseCredentialsProvider() {
		esConfig, err := getConfigFromCredentialsProvider(ctx, forceRefresh)
		if err != nil {
			fmt.Printf("Error getting config from credentials provider: %v\n", err)
			return nil, err
		}
		return esConfig, nil
	} else {
		esConfig, err := getConfigFromEnv()
		if err != nil {
			fmt.Printf("Error getting config from env: %v\n", err)
			return nil, err
		}
		return esConfig, nil
	}
}

// Ping returns whether the Elasticsearch cluster is running.
func (e *Client) Ping() error {
	res, err := e.client.Ping()
	if err != nil {
		fmt.Printf("Error pinging elasticsearch: %v\n", err)
		return fmt.Errorf("failed to ping elasticsearch: %w", err)
	}
	if res.IsError() {
		fmt.Printf("Error pinging elasticsearch: %s\n", res.String())
		return fmt.Errorf("failed to ping elasticsearch: %s", res.String())
	}

	defer res.Body.Close()

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
	req := &esapi.SearchRequest{}

	for _, opt := range o {
		opt(req)
	}

	res, err := req.Do(ctx, e.client)

	if res.IsError() {
		if res.StatusCode == 401 {
			// Unauthorized error, reauthenticate and retry
			err = e.Reauthenticate(ctx)
			if err != nil {
				return nil, fmt.Errorf("error: %s", err)
			}
			res, err = req.Do(ctx, e.client)
			if err != nil {
				return nil, fmt.Errorf("error: %s", err)
			}
		} else {
			return nil, fmt.Errorf("error while querying: %s", res.String())
		}
	}
	return res, err
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
	res, err := req.Do(context.Background(), e.client)
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
	res, err := req.Do(context.Background(), e.client)
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
	res, err := req.Do(context.Background(), e.client)
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
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return nil, fmt.Errorf("error getting elasticsearch information: %s", err)
	}

	result, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	return result, nil
}
