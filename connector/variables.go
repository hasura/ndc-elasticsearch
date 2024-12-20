package connector

import (
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
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
			resultMap := make(map[string]interface{})
			for key, value := range elem {
				res, err := replaceVariables(value, variableSet)
				if err != nil {
					return nil, err
				}
				resultMap[key] = res
			}
			result = append(result, resultMap)
		}
		return result, nil
	case map[string]interface{}:
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
