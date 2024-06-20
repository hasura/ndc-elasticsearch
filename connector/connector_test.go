package connector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/connector"
	"github.com/hasura/ndc-sdk-go/schema"
)

var testCases = []struct {
	name         string
	requestFile  string
	responseFile string
	response     []byte
}{
	{
		name:         "select_query",
		requestFile:  "../testdata/query/select/request.json",
		responseFile: "../testdata/query/select/response.json",
	},
	{
		name:         "select_query_on_array",
		requestFile:  "../testdata/query/select/array_select_request.json",
		responseFile: "../testdata/query/select/array_select_response.json",
	},
	{
		name:         "sort_asc",
		requestFile:  "../testdata/query/sort/sort_asc_request.json",
		responseFile: "../testdata/query/sort/sort_asc_response.json",
	},
	{
		name:         "sort_desc",
		requestFile:  "../testdata/query/sort/sort_desc_request.json",
		responseFile: "../testdata/query/sort/sort_desc_response.json",
	},
	{
		name:         "multi_column_sort",
		requestFile:  "../testdata/query/sort/multi_column_sort_request.json",
		responseFile: "../testdata/query/sort/multi_column_sort_response.json",
	},
	{
		name:         "sort_on_object_field",
		requestFile:  "../testdata/query/sort/sort_on_object_field_request.json",
		responseFile: "../testdata/query/sort/sort_on_object_field_response.json",
	},
	{
		name:         "sort_on_nested_type_with_nesting",
		requestFile:  "../testdata/query/sort/sort_on_nested_type_with_nesting_request.json",
		responseFile: "../testdata/query/sort/sort_on_nested_type_with_nesting_response.json",
	},
	{
		name:         "sort_on_nested_type_field",
		requestFile:  "../testdata/query/sort/sort_on_nested_type_request.json",
		responseFile: "../testdata/query/sort/sort_on_nested_type_response.json",
	},
	{
		name:         "pagination",
		requestFile:  "../testdata/query/pagination/request.json",
		responseFile: "../testdata/query/pagination/response.json",
	},
	{
		name:         "predicate_with_and",
		requestFile:  "../testdata/query/filter/predicate_with_and_request.json",
		responseFile: "../testdata/query/filter/predicate_with_and_response.json",
	},
	{
		name:         "predicate_with_or",
		requestFile:  "../testdata/query/filter/predicate_with_or_request.json",
		responseFile: "../testdata/query/filter/predicate_with_or_response.json",
	},
	{
		name:         "predicate_with_not",
		requestFile:  "../testdata/query/filter/predicate_with_not_request.json",
		responseFile: "../testdata/query/filter/predicate_with_not_response.json",
	},
	{
		name:         "predicate_with_terms",
		requestFile:  "../testdata/query/filter/predicate_with_terms_request.json",
		responseFile: "../testdata/query/filter/predicate_with_terms_response.json",
	},
	{
		name:         "predicate_with_match",
		requestFile:  "../testdata/query/filter/predicate_with_match_request.json",
		responseFile: "../testdata/query/filter/predicate_with_match_response.json",
	},
	{
		name:         "nested_predicate",
		requestFile:  "../testdata/query/filter/nested_predicate_request.json",
		responseFile: "../testdata/query/filter/nested_predicate_response.json",
	},
	{
		name:         "predicate_on_object_field",
		requestFile:  "../testdata/query/filter/predicate_on_object_request.json",
		responseFile: "../testdata/query/filter/predicate_on_object_response.json",
	},
	{
		name:         "predicate_on_nested_type_with_nesting",
		requestFile:  "../testdata/query/filter/predicate_on_nested_type_with_nesting_request.json",
		responseFile: "../testdata/query/filter/predicate_on_nested_type_with_nesting_response.json",
	},
	{
		name:         "predicate_on_nested_type_field",
		requestFile:  "../testdata/query/filter/predicate_on_nested_type_request.json",
		responseFile: "../testdata/query/filter/predicate_on_nested_type_response.json",
	},
	{
		name:         "unary_predicate_on_nested_type_field",
		requestFile:  "../testdata/query/filter/unary_predicate_on_nested_type_request.json",
		responseFile: "../testdata/query/filter/unary_predicate_on_nested_type_response.json",
	},
	{
		name:         "range_query",
		requestFile:  "../testdata/query/filter/range_query_request.json",
		responseFile: "../testdata/query/filter/range_query_response.json",
	},
	{
		name:         "star_count_aggregation",
		requestFile:  "../testdata/query/aggregation/star_count_request.json",
		responseFile: "../testdata/query/aggregation/star_count_response.json",
	},
	{
		name:         "column_count_aggregation",
		requestFile:  "../testdata/query/aggregation/column_count_request.json",
		responseFile: "../testdata/query/aggregation/column_count_response.json",
	},
	{
		name:         "single_column_aggregation",
		requestFile:  "../testdata/query/aggregation/single_column_request.json",
		responseFile: "../testdata/query/aggregation/single_column_response.json",
	},
	// Test cases for variables
	{
		name:         "single_column_aggregation_using_variables",
		requestFile:  "../testdata/query/variables/aggregation/single_column_request.json",
		responseFile: "../testdata/query/variables/aggregation/single_column_response.json",
	},
	{
		name:         "column_count_aggregation_using_variables",
		requestFile:  "../testdata/query/variables/aggregation/column_count_request.json",
		responseFile: "../testdata/query/variables/aggregation/column_count_response.json",
	},
	{
		name:         "star_count_aggregation_using_variables",
		requestFile:  "../testdata/query/variables/aggregation/star_count_request.json",
		responseFile: "../testdata/query/variables/aggregation/star_count_response.json",
	},
	{
		name:         "sort_asc_using_variables",
		requestFile:  "../testdata/query/variables/sort/sort_asc_request.json",
		responseFile: "../testdata/query/variables/sort/sort_asc_response.json",
	},
	{
		name:         "sort_desc_using_variables",
		requestFile:  "../testdata/query/variables/sort/sort_desc_request.json",
		responseFile: "../testdata/query/variables/sort/sort_desc_response.json",
	},
	{
		name:         "multi_column_sort_using_variables",
		requestFile:  "../testdata/query/variables/sort/multi_column_sort_request.json",
		responseFile: "../testdata/query/variables/sort/multi_column_sort_response.json",
	},
	{
		name:         "pagination_using_variables",
		requestFile:  "../testdata/query/variables/pagination/request.json",
		responseFile: "../testdata/query/variables/pagination/response.json",
	},
	{
		name:         "predicate_with_and_using_variables",
		requestFile:  "../testdata/query/variables/filter/predicate_with_and_request.json",
		responseFile: "../testdata/query/variables/filter/predicate_with_and_response.json",
	},
	{
		name:         "predicate_with_or_using_variables",
		requestFile:  "../testdata/query/variables/filter/predicate_with_or_request.json",
		responseFile: "../testdata/query/variables/filter/predicate_with_or_response.json",
	},
	{
		name:         "predicate_with_not_using_variables",
		requestFile:  "../testdata/query/variables/filter/predicate_with_not_request.json",
		responseFile: "../testdata/query/variables/filter/predicate_with_not_response.json",
	},
	{
		name:         "predicate_with_terms_using_variables",
		requestFile:  "../testdata/query/variables/filter/predicate_with_terms_request.json",
		responseFile: "../testdata/query/variables/filter/predicate_with_terms_response.json",
	},
	{
		name:         "predicate_with_match_using_variables",
		requestFile:  "../testdata/query/variables/filter/predicate_with_match_request.json",
		responseFile: "../testdata/query/variables/filter/predicate_with_match_response.json",
	},
	{
		name:         "nested_predicate_using_variables",
		requestFile:  "../testdata/query/variables/filter/nested_predicate_request.json",
		responseFile: "../testdata/query/variables/filter/nested_predicate_response.json",
	},
	{
		name:         "predicate_on_object_field_using_variables",
		requestFile:  "../testdata/query/variables/filter/predicate_on_object_request.json",
		responseFile: "../testdata/query/variables/filter/predicate_on_object_response.json",
	},
	{
		name:         "predicate_on_nested_type_field_using_variables",
		requestFile:  "../testdata/query/variables/filter/predicate_on_nested_type_request.json",
		responseFile: "../testdata/query/variables/filter/predicate_on_nested_type_response.json",
	},
}

// createTestServer creates a test server for the given configuration.
func createTestServer(t *testing.T) *connector.Server[types.Configuration, types.State] {
	server, err := connector.NewServer(&Connector{}, &connector.ServerOptions{
		Configuration: "../testdata",
		InlineConfig:  true,
	}, connector.WithoutRecovery())

	if err != nil {
		t.Errorf("NewServer: expected no error, got %s", err)
		t.FailNow()
	}

	return server
}

// fetchTestSample reads the specified file and returns its content.
func fetchTestSample(t *testing.T, fileName string) []byte {
	res, err := os.ReadFile(fileName)
	if err != nil {
		t.Errorf("failed to read file %s: %s", fileName, err)
		t.FailNow()
	}
	return res
}

// assertHTTPResponseStatus asserts the HTTP response status code.
func assertHTTPResponseStatus(t *testing.T, name string, res *http.Response, statusCode int) {
	if res.StatusCode != statusCode {
		t.Errorf("\n%s: expected status %d, got %d", name, statusCode, res.StatusCode)
		t.FailNow()
	}
}

// assertHTTPResponse performs assertions on the HTTP response.
func assertHTTPResponse[B any](t *testing.T, res *http.Response, statusCode int, expectedBody B) {
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		t.Error("failed to read response body")
		t.FailNow()
	}

	if res.StatusCode != statusCode {
		t.Errorf("expected status %d, got %d. Body: %s", statusCode, res.StatusCode, string(bodyBytes))
		t.FailNow()
	}

	var body B
	if err = json.Unmarshal(bodyBytes, &body); err != nil {
		t.Errorf("failed to decode json body, got error: %s; body: %s", err, string(bodyBytes))
		t.FailNow()
	}

	if !internal.DeepEqual(expectedBody, body) {
		expectedBytes, _ := json.Marshal(expectedBody)
		t.Errorf("\nexpect: %+v\ngot: %+v", string(expectedBytes), string(bodyBytes))
		t.FailNow()
	}
}

// TestGeneralMethods tests various general methods like capabilities, schema, health, and metrics.
func TestGeneralMethods(t *testing.T) {
	server := createTestServer(t).BuildTestServer()
	t.Run("capabilities", func(t *testing.T) {
		expectedBytes, err := os.ReadFile("../testdata/capabilities.json")
		if err != nil {
			t.Errorf("failed to get expected capabilities: %s", err.Error())
			t.FailNow()
		}

		var expectedResult schema.CapabilitiesResponse
		err = json.Unmarshal(expectedBytes, &expectedResult)
		if err != nil {
			t.Errorf("failed to read expected body: %s", err.Error())
			t.FailNow()
		}

		httpResp, err := http.Get(fmt.Sprintf("%s/capabilities", server.URL))
		if err != nil {
			t.Errorf("failed to fetch capabilities: %s", err.Error())
			t.FailNow()
		}

		assertHTTPResponse(t, httpResp, http.StatusOK, expectedResult)
	})

	t.Run("schema", func(t *testing.T) {
		expectedBytes, err := os.ReadFile("../testdata/schema.json")
		if err != nil {
			t.Errorf("failed to fetch expected schema: %s", err.Error())
			t.FailNow()
		}

		var expectedSchema schema.SchemaResponse
		err = json.Unmarshal(expectedBytes, &expectedSchema)
		if err != nil {
			t.Errorf("failed to read expected body: %s", err.Error())
			t.FailNow()
		}

		httpResp, err := http.Get(fmt.Sprintf("%s/schema", server.URL))
		if err != nil {
			t.Errorf("failed to fetch schema: %s", err.Error())
			t.FailNow()
		}

		assertHTTPResponse(t, httpResp, http.StatusOK, expectedSchema)
	})

	t.Run("GET /health", func(t *testing.T) {
		res, err := http.Get(fmt.Sprintf("%s/health", server.URL))
		if err != nil {
			t.Errorf("expected no error, got %s", err)
			t.FailNow()
		}
		assertHTTPResponseStatus(t, "GET /health", res, http.StatusOK)
	})

	t.Run("GET /metrics", func(t *testing.T) {
		res, err := http.Get(fmt.Sprintf("%s/metrics", server.URL))
		if err != nil {
			t.Errorf("expected no error, got %s", err)
			t.FailNow()
		}
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("\n%s: expected 404 got status %d", "/metrics", res.StatusCode)
			t.FailNow()
		}
	})
}

// TestQuery tests various query scenarios by sending HTTP requests and asserting the responses.
func TestQuery(t *testing.T) {
	server := createTestServer(t).BuildTestServer()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := fetchTestSample(t, tc.requestFile)
			var expected schema.QueryResponse
			var err error
			if len(tc.response) > 0 {
				err = json.Unmarshal(tc.response, &expected)
			} else {
				expectedRes := fetchTestSample(t, tc.responseFile)
				err = json.Unmarshal(expectedRes, &expected)
			}
			if err != nil {
				t.Errorf("failed to decode expected response: %s", err)
				t.FailNow()
			}

			res, err := http.Post(fmt.Sprintf("%s/query", server.URL), "application/json", bytes.NewReader(req))
			if err != nil {
				t.Errorf("expected no error, got %s", err)
				t.FailNow()
			}

			assertHTTPResponse(t, res, http.StatusOK, expected)
		})
	}
}
