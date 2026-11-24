package connector

import (
	"encoding/json"
	"testing"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/v2/schema"
	"github.com/stretchr/testify/assert"
)

// variableFilters returns the per-variable-set filters built by executeQueryWithVariables.
func variableFilters(t *testing.T, query map[string]interface{}, variableSets []schema.QueryRequestVariablesElem) string {
	t.Helper()
	result, err := executeQueryWithVariables(variableSets, map[string]interface{}{"query": query})
	assert.NoError(t, err)
	filters := result["aggs"].(map[string]interface{})["result"].(map[string]interface{})["filters"].(map[string]interface{})["filters"]
	encoded, err := json.Marshal(filters)
	assert.NoError(t, err)
	return string(encoded)
}

func TestNullJoinKeyBecomesMatchNone(t *testing.T) {
	query := map[string]interface{}{"term": map[string]interface{}{"employeeId": types.Variable("$employeeId")}}
	variableSets := []schema.QueryRequestVariablesElem{
		{"$employeeId": "id-a"},
		{"$employeeId": nil},
		{"$employeeId": "id-b"},
	}

	assert.JSONEq(t, `[
		{"term": {"employeeId": "id-a"}},
		{"match_none": {}},
		{"term": {"employeeId": "id-b"}}
	]`, variableFilters(t, query, variableSets))
}

func TestAllNullJoinKeysKeepOneFilterPerVariableSet(t *testing.T) {
	query := map[string]interface{}{"term": map[string]interface{}{"employeeId": types.Variable("$employeeId")}}
	variableSets := []schema.QueryRequestVariablesElem{
		{"$employeeId": nil},
		{"$employeeId": nil},
	}

	assert.JSONEq(t, `[{"match_none": {}}, {"match_none": {}}]`, variableFilters(t, query, variableSets))
}

func TestNullJoinKeyInsideNestedQuery(t *testing.T) {
	query := map[string]interface{}{
		"nested": map[string]interface{}{
			"path":  "officers",
			"query": map[string]interface{}{"term": map[string]interface{}{"officers.employeeId": types.Variable("$employeeId")}},
		},
	}
	variableSets := []schema.QueryRequestVariablesElem{
		{"$employeeId": nil},
		{"$employeeId": "id-a"},
	}

	assert.JSONEq(t, `[
		{"nested": {"path": "officers", "query": {"match_none": {}}}},
		{"nested": {"path": "officers", "query": {"term": {"officers.employeeId": "id-a"}}}}
	]`, variableFilters(t, query, variableSets))
}

func TestNullJoinKeyInsideCompositeKey(t *testing.T) {
	query := map[string]interface{}{
		"bool": map[string]interface{}{
			"must": []map[string]interface{}{
				{"term": map[string]interface{}{"region": types.Variable("$region")}},
				{"term": map[string]interface{}{"employeeId": types.Variable("$employeeId")}},
			},
		},
	}
	variableSets := []schema.QueryRequestVariablesElem{
		{"$region": "emea", "$employeeId": nil},
		{"$region": "emea", "$employeeId": "id-a"},
	}

	assert.JSONEq(t, `[
		{"bool": {"must": [{"term": {"region": "emea"}}, {"match_none": {}}]}},
		{"bool": {"must": [{"term": {"region": "emea"}}, {"term": {"employeeId": "id-a"}}]}}
	]`, variableFilters(t, query, variableSets))
}

func TestMissingVariableIsStillAnError(t *testing.T) {
	query := map[string]interface{}{"term": map[string]interface{}{"employeeId": types.Variable("$employeeId")}}
	_, err := executeQueryWithVariables([]schema.QueryRequestVariablesElem{{"$other": "x"}}, map[string]interface{}{"query": query})
	assert.Error(t, err)
}

func TestNonNullFalsyJoinKeysAreKept(t *testing.T) {
	query := map[string]interface{}{"term": map[string]interface{}{"employeeId": types.Variable("$employeeId")}}
	variableSets := []schema.QueryRequestVariablesElem{
		{"$employeeId": ""},
		{"$employeeId": 0},
		{"$employeeId": false},
	}

	assert.JSONEq(t, `[
		{"term": {"employeeId": ""}},
		{"term": {"employeeId": 0}},
		{"term": {"employeeId": false}}
	]`, variableFilters(t, query, variableSets))
}
