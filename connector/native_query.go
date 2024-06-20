package connector

import (
	"context"
	"regexp"

	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// handleNativeQuery is a function that processes a native query by reading and parsing it from a file,
// replacing arguments in the template, and preparing the final native query.
func handleNativeQuery(
	ctx context.Context,
	queryConfig types.NativeQuery,
	query map[string]interface{},
	arguments map[string]schema.Argument,
) (map[string]interface{}, error) {
	dsl := queryConfig.DSL

	nativeQuery := make(map[string]interface{})
	var err error
	if dsl.File != nil && *dsl.File != "" {
		nativeQuery, err = internal.ReadJsonFileUsingDecoder(*dsl.File)
		if err != nil {
			return nil, err
		}
	} else if dsl.Internal != nil {
		nativeQuery = *dsl.Internal
	}

	if queryConfig.Arguments != nil {
		nativeQuery, err = processArguments(nativeQuery, arguments, *queryConfig.Arguments)
		if err != nil {
			return nil, err
		}
	}

	return prepareNativeQuery(ctx, nativeQuery, query), nil
}

// prepareNativeQuery prepares the native query based on the input query parameters.
// It merges the aggregates and filters from the native query into the main query.
func prepareNativeQuery(
	ctx context.Context,
	nativeQuery map[string]interface{},
	query map[string]interface{},
) map[string]interface{} {
	postProcessor := ctx.Value("postProcessor").(*types.PostProcessor)

	// Merge aggregates
	if aggs, ok := nativeQuery["aggs"].(map[string]interface{}); ok {
		if aggregates, ok := query["aggs"].(map[string]interface{}); ok {
			for aggregateName, aggregate := range aggs {
				postProcessor.ColumnAggregate[aggregateName] = false
				aggregates[aggregateName] = aggregate
			}
		} else {
			query["aggs"] = aggs
		}
	}

	// Merge filters
	if nqFilters, ok := nativeQuery["query"]; ok {
		if filters, ok := query["query"]; ok {
			// append native query filters to the boolean query if already present
			if boolFilter, ok := filters.(map[string]interface{})["bool"]; ok {
				if mustFilter, ok := boolFilter.(map[string]interface{})["must"].([]interface{}); ok {
					query["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"] = append(mustFilter, nqFilters)
				}
			} else {
				// create new boolean query if not present
				query["query"] = map[string]interface{}{
					"bool": map[string]interface{}{
						"must": []interface{}{filters, nqFilters},
					},
				}
			}
		}
	}

	return query
}

// replacePlaceholders recursively replaces placeholders in the template map with actual values
func replaceArguments(template interface{}, arguments map[string]schema.Argument) (interface{}, error) {
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)

	switch v := template.(type) {
	case map[string]interface{}:
		for key, value := range v {
			replacedValue, err := replaceArguments(value, arguments)
			if err != nil {
				return nil, err
			}
			v[key] = replacedValue
		}
		return v, nil
	case []interface{}:
		for i, item := range v {
			replacedItem, err := replaceArguments(item, arguments)
			if err != nil {
				return nil, err
			}
			v[i] = replacedItem
		}
		return v, nil
	case string:
		placeholder := re.FindStringSubmatch(v)
		if len(placeholder) > 0 {
			arg := arguments[placeholder[1]]
			argValue, err := evalArgument(&arg)
			if err != nil {
				return nil, err
			}
			return argValue, nil
		}
		return v, nil
	default:
		return v, nil
	}
}

// ProcessArguments replaces placeholders in the template with values and returns the processed template
func processArguments(
	template map[string]interface{},
	arguments map[string]schema.Argument,
	queryArgs map[string]interface{},
) (map[string]interface{}, error) {
	err := validateArguments(queryArgs, arguments)
	if err != nil {
		return nil, err
	}

	if len(arguments) > 0 {
		processedTemplate, err := replaceArguments(template, arguments)
		if err != nil {
			return nil, err
		}
		return processedTemplate.(map[string]interface{}), nil
	}

	return template, nil
}
