package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// prepareAggregateQuery prepares the aggregate query based on the aggregates in the query request.
func prepareAggregateQuery(ctx context.Context, aggregates schema.QueryAggregates, state *types.State) (map[string]interface{}, error) {
	postProcessor := ctx.Value("postProcessor").(*types.PostProcessor)
	aggs := make(map[string]interface{})
	for name, aggregate := range aggregates {
		aggregatColumn, ok := aggregate["column"].(string)
		if ok {
			if _, ok := state.UnsupportedAggregateFields[aggregatColumn]; ok {
				return nil, schema.BadRequestError("aggregation not supported on this field", map[string]any{
					"value": aggregatColumn,
				})
			}
		}
		switch agg := aggregate.Interface().(type) {
		case *schema.AggregateStarCount:
			postProcessor.StarAggregates = name
		case *schema.AggregateColumnCount:
			if agg.Distinct {
				aggs[name] = map[string]interface{}{
					"cardinality": map[string]interface{}{
						"field": agg.Column,
					},
				}
			} else {
				aggs[name] = map[string]interface{}{
					"filter": map[string]interface{}{
						"exists": map[string]interface{}{
							"field": agg.Column,
						},
					},
				}
			}
			postProcessor.ColumnAggregate = append(postProcessor.ColumnAggregate, name)
		case *schema.AggregateSingleColumn:
			postProcessor.ColumnAggregate = append(postProcessor.ColumnAggregate, name)
			switch agg.Function {
			case "sum", "min", "max", "avg", "value_count", "cardinality", "stats", "string_stats":
				aggs[name] = map[string]interface{}{
					agg.Function: map[string]interface{}{
						"field": agg.Column,
					},
				}
			default:
				return nil, schema.BadRequestError("invalid aggregate function", map[string]any{
					"value": agg.Function,
				})
			}
		default:
			return nil, schema.BadRequestError("invalid aggregate field", map[string]any{
				"value": aggregate["type"],
			})
		}
	}

	return aggs, nil
}
