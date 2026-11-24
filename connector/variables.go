package connector

import (
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/v2/schema"
)

// executeQueryWithVariables prepares a dsl query for query with variables.
// It takes a list of variable sets and a query body as input.
// It replaces the variable names in the query body with values from the variable sets.
// It returns the prepared query and an error if any.
func executeQueryWithVariables(variableSets []schema.QueryRequestVariablesElem, body map[string]interface{}) (map[string]interface{}, error) {
	variableQuery := make(map[string]interface{})
	// do not to return any documents in the search results while performing aggregations
	variableQuery["size"] = 0

	// 100 is the default max result size limit (per bucket) for top_hits aggregation
	// This limit can be set by changing the [index.max_inner_result_window] index level setting.
	// TODO: we should read this setting and set the limit accordingly
	// A `bucket` here refers to a group of documents that match a certain clause/perdicate, and the top_hits aggregation can have multiple clauses/predicates
	const TOP_HITS_MAX_BUCKET_RESULT_SIZE = 100

	var filters []interface{}
	if filter, ok := body["query"]; ok {
		for _, variableSet := range variableSets {
			// Replace variable names in the filter map with values from variableSet
			updatedFilter, err := replaceVariables(filter, variableSet)
			if err != nil {
				return nil, err
			}
			filters = append(filters, updatedFilter)
		}
	}

	topHits := make(map[string]interface{})
	topHits["_source"] = body["_source"]
	topHits["size"] = TOP_HITS_MAX_BUCKET_RESULT_SIZE
	if size, ok := body["size"]; ok {
		if (size.(int)) > TOP_HITS_MAX_BUCKET_RESULT_SIZE {
			topHits["size"] = TOP_HITS_MAX_BUCKET_RESULT_SIZE
		} else {
			topHits["size"] = size
		}
	}
	if limit, ok := body["limit"]; ok {
		topHits["from"] = limit
	}
	if sort, ok := body["sort"]; ok {
		topHits["sort"] = sort
	}
	aggregate := make(map[string]interface{})
	if aggs, ok := body["aggs"].(map[string]interface{}); ok {
		aggregate = aggs
	}
	aggregate["docs"] = map[string]interface{}{
		"top_hits": topHits,
	}

	variableQuery["aggs"] = map[string]interface{}{
		"result": map[string]interface{}{
			"filters": map[string]interface{}{
				"filters": filters,
			},
			"aggs": aggregate,
		},
	}

	return variableQuery, nil
}

// replaceVariables replaces variable names in the filter with values from variableSet
func replaceVariables(input interface{}, variableSet map[string]interface{}) (interface{}, error) {
	switch value := input.(type) {
	case types.Variable:
		if replacement, ok := variableSet[string(value)]; ok {
			return replacement, nil
		}
		return nil, schema.UnprocessableContentError("variable not found in variable set", map[string]interface{}{"variable": string(value)})
	case []interface{}:
		result := make([]interface{}, len(value))
		for i, elem := range value {
			res, err := replaceVariables(elem, variableSet)
			if err != nil {
				return nil, err
			}
			result[i] = res
		}
		return result, nil
	case []map[string]interface{}:
		var result []map[string]interface{}
		for _, elem := range value {
			res, err := replaceVariables(elem, variableSet)
			if err != nil {
				return nil, err
			}
			resultMap, ok := res.(map[string]interface{})
			if !ok {
				return nil, schema.UnprocessableContentError("invalid filter clause", map[string]interface{}{"value": res})
			}
			result = append(result, resultMap)
		}
		return result, nil
	case map[string]interface{}:
		if comparesWithNullVariable(value, variableSet) {
			// A comparison against null never matches a document. Elasticsearch
			// rejects these leaf queries with a null value, and one rejected
			// filter fails every variable set in the request, so replace the
			// leaf with a query that matches nothing. The filter keeps its
			// position, so the other variable sets still get their own rows.
			return map[string]interface{}{"match_none": map[string]interface{}{}}, nil
		}
		result := make(map[string]interface{})
		for key, value := range value {
			res, err := replaceVariables(value, variableSet)
			if err != nil {
				return nil, err
			}
			result[key] = res
		}
		return result, nil
	default:
		return input, nil
	}
}

// singleValueQueries are the Elasticsearch leaf queries that compare one field
// against a value supplied by a comparison operator.
var singleValueQueries = map[string]struct{}{
	"term":                {},
	"terms":               {},
	"match":               {},
	"match_phrase":        {},
	"match_phrase_prefix": {},
	"match_bool_prefix":   {},
	"prefix":              {},
	"wildcard":            {},
	"regexp":              {},
}

// comparesWithNullVariable reports whether query is a leaf query such as
// {"term": {"field": $var}} whose variable is null in variableSet.
// A variable that is missing from the set is not treated as null, so that
// replaceVariables still reports it as an error.
func comparesWithNullVariable(query map[string]interface{}, variableSet map[string]interface{}) bool {
	if len(query) != 1 {
		return false
	}
	for operator, operand := range query {
		if _, ok := singleValueQueries[operator]; !ok {
			return false
		}
		fieldValue, ok := operand.(map[string]interface{})
		if !ok || len(fieldValue) != 1 {
			return false
		}
		for _, value := range fieldValue {
			variable, ok := value.(types.Variable)
			if !ok {
				return false
			}
			replacement, found := variableSet[string(variable)]
			return found && replacement == nil
		}
	}
	return false
}
