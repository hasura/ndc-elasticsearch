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

func (e *Client) Search(ctx context.Context, index string, body map[string]interface{}) (map[string]interface{}, error) {
	logger := connector.GetLogger(ctx)
	es := e.client

	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}

	res, err := es.Search(
		es.Search.WithContext(ctx),
		es.Search.WithIndex(index),
		es.Search.WithBody(&buf),
	)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return nil, fmt.Errorf("error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			root_cause, _ :=  e["error"].(map[string]interface{})["root_cause"].([]interface {})[0].(map[string]interface{})
			errMsg := fmt.Sprintf("[%s] %s: %s",
				res.Status(),
				root_cause["type"],
				root_cause["reason"],
			)
			logger.DebugContext(ctx, "Response Details", "response", e)
			return nil, errors.New(errMsg)
		}
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error parsing the response body: %s", err)
	}

	return result, nil
}

// GetIndices Returns comma seperated list of indices that does not start with `.` character
func (e *Client) GetIndices() ([]string, error) {
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

	indices, err := parseResponse(res)
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
func (e *Client) GetMappings(indices []string) (interface{}, error) {
	req := esapi.IndicesGetMappingRequest{
		Index: indices,
	}

	// Perform the request
	res, err := req.Do(context.Background(), e.client)
	if err != nil {
		return nil, fmt.Errorf("error getting mappings: %s", err)
	}

	result, err := parseResponse(res)
	if err != nil {
		return nil, err
	}

	return result, nil

}

func parseResponse(res *esapi.Response) (interface{}, error) {
	defer res.Body.Close()
	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return nil, fmt.Errorf("error parsing the response body: %s", err)
		} else {
			_type, _ := e["error"].(map[string]interface{})["type"]
			reason, _ := e["error"].(map[string]interface{})["reason"]
			errMsg := fmt.Sprintf("[%s] %s: %s", res.Status(), _type, reason)
			return nil, errors.New(errMsg)
		}
	}

	var result any
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error parsing the response body: %s", err)
	}

	return result, nil
}
