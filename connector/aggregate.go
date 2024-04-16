package connector

import (
	"context"

	"github.com/hasura/ndc-sdk-go/schema"
)

func prepareAggregateQuery(ctx context.Context, aggregates schema.QueryAggregates) (map[string]interface{}, error) {
	sanitizer := ctx.Value("sanitizer").(*Sanitizer)
	aggs := make(map[string]interface{})
	for name, aggregate := range aggregates {
		switch aggregate["type"] {
		case schema.AggregateTypeStarCount:
			sanitizer.startAggregates = name
		case schema.AggregateTypeColumnCount:
			agg, err := aggregate.AsColumnCount()
			if err != nil {
				return nil, err
			}
			function := "value_count"
			if agg.Distinct {
				function = "cardinality"
			}
			sanitizer.columnCount = append(sanitizer.columnCount, name)
			aggs[name] = map[string]interface{}{
				function: map[string]interface{}{
					"field": agg.Column,
				},
			}
		default:
			return nil, schema.UnprocessableContentError("Unsupported aggregate type", map[string]interface{}{
				"value": aggregate["type"],
			})
		}
	}

	return aggs, nil
}
