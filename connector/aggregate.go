package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

func prepareAggregateQuery(ctx context.Context, aggregates schema.QueryAggregates) (map[string]interface{}, error) {
	postProcessor := ctx.Value("postProcessor").(*types.PostProcessor)
	aggs := make(map[string]interface{})
	for name, aggregate := range aggregates {
		switch aggregate["type"] {
		case schema.AggregateTypeStarCount:
			postProcessor.StarAggregates = name
		case schema.AggregateTypeColumnCount:
			agg, err := aggregate.AsColumnCount()
			if err != nil {
				return nil, err
			}
			function := "value_count"
			if agg.Distinct {
				function = "cardinality"
			}
			postProcessor.ColumnCount = append(postProcessor.ColumnCount, name)
			aggs[name] = map[string]interface{}{
				function: map[string]interface{}{
					"field": agg.Column,
				},
			}
		default:
			return nil, schema.UnprocessableContentError("invalid aggregate field", map[string]interface{}{
				"value": aggregate["type"],
			})
		}
	}

	return aggs, nil
}
