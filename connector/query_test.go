package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
	"github.com/stretchr/testify/assert"
)

const testsPath = "../testdata/unit_tests/query_tests/"

type test struct {
	group      string
	name       string
	ndcRequest *schema.QueryRequest
	state      *types.State
	wantQuery  []byte
}

var tests = []test{
	{
		group: "payments",
		name:  "simple_query",
	},
	{
		group: "payments",
		name:  "simple_query_with_limit",
	},
	{
		group: "payments",
		name:  "nested_query",
	},
	{
		group: "payments",
		name:  "nested_query_with_limit",
	},
	{
		group: "payments",
		name:  "sort_by_type",
	},
	{
		group: "payments",
		name:  "sort_by_subtype",
	},
	{
		group: "payments",
		name:  "count_aggregation",
	},
	{
		group: "payments",
		name:  "nested_aggregations",
	},
	{
		group: "payments",
		name:  "query_with_offset",
	},
	{
		group: "payments",
		name:  "query_with_variables",
	},
	{
		group: "payments",
		name:  "simple_where_clause",
	},
	{
		group: "payments",
		name:  "simple_where_not_clause",
	},
	{
		group: "payments",
		name:  "simple_and_clause",
	},
	{
		group: "payments",
		name:  "simple_or_clause",
	},
	{
		group: "payments",
		name:  "simple_terms_clause",
	},
	{
		group: "payments",
		name:  "simple_subtype_where_clause",
	},
	{
		group: "customers",
		name:  "sort_by_subtype",
	},
	{
		group: "payments",
		name:  "multiple_aggregations",
	},
	{
		group: "payments",
		name:  "multiple_aggregations_with_where_clause",
	},
	{
		group: "payments",
		name:  "float_aggregations",
	},
	{
		group: "payments",
		name:  "float_aggregations_with_range",
	},
	{
		group: "customers",
		name:  "simple_subtype_where_clause",
	},
	{
		group: "customers",
		name:  "subtype_term_clause",
	},
	{
		group: "payments",
		name:  "cardinality_aggregation",
	},
	{
		group: "payments",
		name:  "count_distinct_aggregation",
	},
	{
		group: "payments",
		name:  "subfield_cardinality_aggregation",
	},
}

func TestPrepareElasticsearchQuery(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.group+"."+tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = context.WithValue(ctx, "postProcessor", &types.PostProcessor{})
			initTest(t, &tt)

			assert.NotNil(t, tt.state, "state is nil")
			assert.NotNil(t, tt.state.Configuration, "configuration is nil")
			assert.NotNil(t, tt.ndcRequest, "ndcRequest is nil")

			ParseConfigurationToSchema(tt.state.Configuration, tt.state)

			query, err := prepareElasticsearchQuery(ctx, tt.ndcRequest, tt.state, tt.ndcRequest.Collection)
			// this correction is added because sometimes the order of _source array would change which resulted in the tests being flaky
			assert.NoError(t, err, "Error preparing query")
			sortSourceArray(query)
			assert.NoError(t, err, "Error sorting selected columns")

			// handle variables
			if len(tt.ndcRequest.Variables) != 0 {
				query, err = executeQueryWithVariables(tt.ndcRequest.Variables, query)
				assert.NoError(t, err, "Error preparing query with variables")
			}

			queryJson, err := json.MarshalIndent(query, "", "  ")
			assert.NoError(t, err, "Error marshalling query")

			assert.JSONEq(t, string(tt.wantQuery), string(queryJson))

			// uncomment to update want file
			// err = os.WriteFile(filepath.Join(testsPath, tt.group, tt.name, "want.json"), queryJson, 0644)
			// assert.NoError(t, err, "Error writing want file")
		})
	}
}

func initTest(t *testing.T, testCase *test) {
	configurationB, err := os.ReadFile(filepath.Join(testsPath, testCase.group, "configuration.json"))
	assert.NoError(t, err, "Error reading configuration file")

	var configuration types.Configuration
	err = json.Unmarshal(configurationB, &configuration)
	assert.NoError(t, err, "Error unmarshalling configuration")

	testCase.state = &types.State{
		Client:                   nil,
		SupportedSortFields:      make(map[string]interface{}),
		SupportedAggregateFields: make(map[string]interface{}),
		SupportedFilterFields:    make(map[string]interface{}),
		ElasticsearchInfo:        make(map[string]interface{}),
		NestedFields:             make(map[string]interface{}),
		Schema:                   nil, // Assuming Tracer is an interface, set to nil or an empty implementation
		Configuration:            &configuration,
	}

	ndcReqeustB, err := os.ReadFile(filepath.Join(testsPath, testCase.group, testCase.name, "ndc_request.json"))
	assert.NoError(t, err, "Error reading ndc_request file")

	err = json.Unmarshal(ndcReqeustB, &testCase.ndcRequest)
	assert.NoError(t, err, "Error unmarshalling ndc_request")

	testCase.wantQuery, err = os.ReadFile(filepath.Join(testsPath, testCase.group, testCase.name, "want.json"))
	assert.NoError(t, err, "Error reading want_query file")
}

// A helper function to sort the _source array in the query
//
// Required because the order of the _source array in the query is not fixed, and the tests were flaky due to this
func sortSourceArray(query map[string]interface{}) (map[string]interface{}, error) {
	source, ok := query["_source"].([]string)
	if !ok {
		return nil, fmt.Errorf("expected _source to be of type []string, got %T", query["_source"])
	}

	sort.Strings(source)
	query["_source"] = source
	return query, nil
}
