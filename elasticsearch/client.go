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
)

type Client struct {
	client *elasticsearch.Client
}

// NewClient creates a new Client
func NewClient() (*Client, error) {
	config, err := getConfigFromEnv()
	if err != nil {
		return nil, err
	}

	c, err := elasticsearch.NewClient(*config)
	if err != nil {
		return nil, err
	}

	client := &Client{client: c}
	err = client.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to validate elasticsearch credentials: %v", err)
	}
	return client, nil
}

// Ping checks if the Client is up and running
func (e *Client) Ping() error {
	res, err := e.client.Ping()
	if err != nil {
		return err
	}

	defer res.Body.Close()

	// Check response status
	if res.IsError() {
		return errors.New(res.String())
	}

	return nil
}

// Search performs a search operation in elastic search.
func (e *Client) Search(ctx context.Context, index string, body map[string]interface{}) (map[string]interface{}, error) {
	es := e.client

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}
	req := esapi.SearchRequest{
		Index: []string{index},
		Body:  &buf,
	}

	res, err := req.Do(ctx, es)
	if err != nil {
		return nil, err
	}

	result, err := parseResponse(ctx, res)
	if err != nil {
		return nil, err
	}

	return result.(map[string]interface{}), nil
}

// GetIndices Returns comma seperated list of indices that does not start with `.` character
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

	// Print the indices matching the regex pattern
	for _, index := range indices.([]interface{}) {
		indexName, ok := index.(map[string]interface{})["index"]
		if !ok {
			continue
		}
		result = append(result, indexName.(string))
	}

	return result, nil
}

// GetMappings Returns mappings for comma seperated list of indices
func (e *Client) GetMappings(ctx context.Context, indices []string) (interface{}, error) {
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

	return result, nil

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

// GetInfo retrieves information about elastic search.
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
