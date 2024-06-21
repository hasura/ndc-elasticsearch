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
		validField := internal.ValidateFieldOperation(state, "aggregate", collection, aggregationColumn)

		if validField == "" {
			return nil, schema.UnprocessableContentError("aggregation not supported on this field", map[string]any{
				"value": aggregationColumn,
			})
		}
		aggregationColumn = validField

		postProcessor.ColumnAggregate[aggregationName] = false
		aggregation, err := prepareAggregate(ctx, aggregationName, aggregation, aggregationColumn, path)
		if err != nil {
			return nil, err
		}
		aggregations[aggregationName] = aggregation
		path = ""
	}

	return aggregations, nil
}

// prepareAggregate prepares the columnCount and SingleColumn query based on the aggregates in the query request.
func prepareAggregate(ctx context.Context, aggName string, agg schema.Aggregate, column string, path string) (map[string]interface{}, error) {
	var aggregation map[string]interface{}
	switch a := agg.Interface().(type) {
	case *schema.AggregateColumnCount:
		aggregation = prepareAggregateColumnCount(ctx, column, path, a.Distinct, aggName)
	case *schema.AggregateSingleColumn:
		var err error
		aggregation, err = prepareAggregateSingleColumn(ctx, a.Function, column, path, aggName)
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
func prepareAggregateColumnCount(ctx context.Context, field string, path string, isDistinct bool, aggName string) map[string]interface{} {
	// Prepare the base aggregation query
	aggregation := map[string]interface{}{
		"field": field,
	}

	// If distinct flag is set, count distinct values
	if isDistinct {
		aggregation = map[string]interface{}{
			"cardinality": aggregation,
		}
	} else {
		// Otherwise, count all occurrences
		aggregation = map[string]interface{}{
			"filter": map[string]interface{}{
				"exists": aggregation,
			},
		}
	}

	// If the field is nested, generate a nested query
	if path != "" {
		aggregation = prepareNestedAggregate(ctx, aggName, aggregation, path)
	}

	return aggregation
}

// prepareAggregateSingleColumn prepares the single column query based on the aggregates in the query request.
// If the field is nested, it generates a nested query to perform the specified function on the field in the nested document.
func prepareAggregateSingleColumn(ctx context.Context, function, field string, path string, aggName string) (map[string]interface{}, error) {
	// Validate the function
	if !internal.Contains(validFunctions, function) {
		return nil, schema.UnprocessableContentError("invalid aggregate function", map[string]any{
			"value": function,
		})
	}

	// Prepare the aggregation query
	aggregation := map[string]interface{}{
		function: map[string]interface{}{
			"field": field,
		},
	}

	// If the field is nested, generate a nested query
	if path != "" {
		aggregation = prepareNestedAggregate(ctx, aggName, aggregation, path)
	}

	return aggregation, nil
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
