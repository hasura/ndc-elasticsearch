package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// prepareAggregateQuery prepares the aggregation query based on the aggregates in the query request.
func prepareAggregateQuery(ctx context.Context, aggregates schema.QueryAggregates, state *types.State, collection string) (map[string]interface{}, error) {
	var path string
	aggregations := make(map[string]interface{})
	postProcessor := ctx.Value("postProcessor").(*types.PostProcessor)
	postProcessor.ColumnAggregate = make(map[string]bool)

	for aggregationName, aggregation := range aggregates {
		aggregationType, err := aggregation.Type()
		if err != nil {
			return nil, err
		}

		if aggregationType == schema.AggregateTypeStarCount {
			postProcessor.StarAggregates = aggregationName
			continue
		}

		aggregationColumn := aggregation["column"].(string)
		fieldPath, ok := aggregation["field_path"].([]string)
		if ok {
			aggregationColumn, path = joinFieldPath(state, fieldPath, aggregationColumn, collection)
		}
		validField := internal.ValidateAggregateOperation(state.SupportedAggregateFields, collection, aggregationColumn)

		if validField == "" {
			return nil, schema.UnprocessableContentError("aggregation not supported on this field", map[string]any{
				"value": aggregationColumn,
			})
		}
		aggregationColumn = validField

		postProcessor.ColumnAggregate[aggregationName] = false
		aggregation, err := prepareAggregate(ctx, state, aggregationName, aggregation, collection, aggregationColumn, path)
		if err != nil {
			return nil, err
		}
		aggregations[aggregationName] = aggregation
		path = ""
	}

	return aggregations, nil
}

// prepareAggregate prepares the columnCount and SingleColumn query based on the aggregates in the query request.
func prepareAggregate(ctx context.Context, state *types.State, aggName string, agg schema.Aggregate, collection string, column string, path string) (map[string]interface{}, error) {
	var aggregation map[string]interface{}
	var err error
	switch a := agg.Interface().(type) {
	case *schema.AggregateColumnCount:
		aggregation, err = prepareAggregateColumnCount(ctx, state, collection, column, path, a.Distinct, aggName)
		if err != nil {
			return nil, err
		}
	case *schema.AggregateSingleColumn:
		aggregation, err = prepareAggregateSingleColumn(ctx, state, a.Function, collection, column, path, aggName)
		if err != nil {
			return nil, err
		}
	default:
		return nil, schema.UnprocessableContentError("invalid aggregate field", map[string]any{
			"value": agg["type"],
		})
	}
	return aggregation, nil
}

// prepareAggregateColumnCount prepares the column count query based on the aggregates in the query request.
// If the field is nested, it generates a nested query to count the occurrences of the field in the nested document.
func prepareAggregateColumnCount(ctx context.Context, state *types.State, collection string, field string, path string, isDistinct bool, aggName string) (map[string]interface{}, error) {
	// the aggregation query
	var aggregation map[string]interface{}

	// If distinct flag is set, count distinct values
	if isDistinct {
		bestFieldOrSubField, err := getCorrectFieldOrSubFieldForFunction(state, collection, field, "cardinality")
		if err != nil {
			return nil, err
		}
		aggregation = map[string]interface{}{
			"cardinality": map[string]interface{}{
				"field": bestFieldOrSubField,
			},
		}
	} else {
		// Otherwise, count all occurrences
		aggregation = map[string]interface{}{
			"filter": map[string]interface{}{
				"exists": map[string]interface{}{
					"field": field,
				},
			},
		}
	}

	// If the field is nested, generate a nested query
	if path != "" {
		aggregation = prepareNestedAggregate(ctx, aggName, aggregation, path)
	}

	return aggregation, nil
}

// prepareAggregateSingleColumn prepares the single column query based on the aggregates in the query request.
// If the field is nested, it generates a nested query to perform the specified function on the field in the nested document.
func prepareAggregateSingleColumn(ctx context.Context, state *types.State, function, collection, field string, path string, aggName string) (map[string]interface{}, error) {
	// Validate the function
	if !internal.Contains(internal.ValidFunctions, function) {
		return nil, schema.UnprocessableContentError("invalid aggregate function", map[string]any{
			"value": function,
		})
	}

	bestFieldOrSubField, err := getCorrectFieldOrSubFieldForFunction(state, collection, field, function)
	if err != nil {
		return nil, err
	}
	// Prepare the aggregation query
	aggregation := map[string]interface{}{
		function: map[string]interface{}{
			"field": bestFieldOrSubField,
		},
	}

	// If the field is nested, generate a nested query
	if path != "" {
		aggregation = prepareNestedAggregate(ctx, aggName, aggregation, path)
	}

	return aggregation, nil
}

func getCorrectFieldOrSubFieldForFunction(state *types.State, collection, field string, function string) (string, error) {
	fType, subFieldMap, _, _ := state.Configuration.GetFieldProperties(collection, field)

	bestFieldOrSubFieldFound := false
	var bestFieldOrSubField string
	operatorFound := false

	if internal.NumericalAggregations[function] {
		bestFieldOrSubField, bestFieldOrSubFieldFound = internal.GetBestFieldOrSubFieldForFamily(field, fType, subFieldMap, internal.NumericFamilyOfTypes)
		operatorFound = true
	}
	if internal.TermLevelAggregations[function] && !bestFieldOrSubFieldFound {
		bestFieldOrSubField, bestFieldOrSubFieldFound = internal.GetBestFieldOrSubFieldForFamily(field, fType, subFieldMap, internal.KeywordFamilyOfTypes)
		operatorFound = true
	}
	if internal.FullTextAggregations[function] && !bestFieldOrSubFieldFound {
		bestFieldOrSubField, _ = internal.GetBestFieldOrSubFieldForFamily(field, fType, subFieldMap, internal.TextFamilyOfTypes)
		operatorFound = true
	}
	if !operatorFound {
		return "", schema.UnprocessableContentError("invalid aggregation function", map[string]any{
			"function": function,
		})
	}
	return bestFieldOrSubField, nil
}

// prepareNestedAggregate generates a nested query to perform the specified function on the field in the nested document.
// The generated query is added to the aggregation map.
func prepareNestedAggregate(ctx context.Context, aggName string, aggregation map[string]interface{}, path string) map[string]interface{} {
	// Update the postProcessor to indicate that the aggregation is on a nested field
	postProcessor := ctx.Value("postProcessor").(*types.PostProcessor)
	postProcessor.ColumnAggregate[aggName] = true

	aggregation = map[string]interface{}{
		"aggs": map[string]interface{}{
			aggName: aggregation,
		},
	}

	// Add the path to the nested field
	aggregation["nested"] = map[string]interface{}{
		"path": path,
	}

	return aggregation
}
