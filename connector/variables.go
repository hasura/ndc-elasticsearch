package connector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

func executeQueryWithVariables(ctx context.Context, state *types.State, variableSets []schema.QueryRequestVariablesElem, body map[string]interface{}) (map[string]interface{}, error) {
	variableQuery := make(map[string]interface{})
	variableQuery["size"] = 0

	var filters []interface{}
	if filter, ok := body["query"]; ok {
		for _, variableSet := range variableSets {
			// Replace variable names in the map with values from memoization
			updatedFilter, err := replaceVariables(filter, variableSet)
			if err != nil {
				return nil, err
			}
			filters = append(filters, updatedFilter)
		}
	}

	topHits := make(map[string]interface{})
	topHits["_source"] = body["_source"]
	topHits["size"] = 10
	if size, ok := body["size"]; ok {
		topHits["size"] = size
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
	// Pretty print query
	queryJSON, _ := json.MarshalIndent(variableQuery, "", "  ")
	fmt.Println("Variable Query", string(queryJSON))

	return variableQuery, nil
}

// Replace variable names in the filter with values from variableSet
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
