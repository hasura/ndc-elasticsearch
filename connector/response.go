package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

func prepareResponseWithVariables(ctx context.Context, res map[string]interface{}) []schema.RowSet {
	rowSets := make([]schema.RowSet, 0)
	postProcessor, ok := ctx.Value("postProcessor").(*types.PostProcessor)
	if !ok {
		return rowSets
	}
	aggregations, ok := res["aggregations"].(map[string]interface{})
	if !ok {
		return rowSets
	}
	result, ok := aggregations["result"].(map[string]interface{})
	if !ok {
		return rowSets
	}
	buckets, ok := result["buckets"].([]interface{})
	if !ok {
		return rowSets
	}
	for _, bucket := range buckets {
		bucketData, ok := bucket.(map[string]interface{})
		if !ok {
			continue
		}
		docs, ok := bucketData["docs"].(map[string]interface{})
		if !ok {
			continue
		}
		rowSet := prepareResponse(ctx, docs)

		if len(postProcessor.ColumnAggregate) != 0 {
			for _, column := range postProcessor.ColumnAggregate {
				if record, ok := bucketData[column].(map[string]interface{}); ok {
					val, ok := record["value"]
					if ok {
						rowSet.Aggregates[column] = val
					} else if val, ok := record["doc_count"]; ok {
						rowSet.Aggregates[column] = val
					} else {
						rowSet.Aggregates[column] = record
					}
				}
			}
		}
		rowSets = append(rowSets, *rowSet)
	}
	return rowSets
}

func prepareResponse(ctx context.Context, res map[string]interface{}) *schema.RowSet {
	postProcessor := ctx.Value("postProcessor").(*types.PostProcessor)
	total := res["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)
	hits := res["hits"].(map[string]interface{})["hits"].([]interface{})
	documents := make([]map[string]interface{}, len(hits))
	for i, hit := range hits {
		doc := hit.(map[string]interface{})
		document := make(map[string]interface{}, len(postProcessor.SelectedFields))
		source := doc["_source"].(map[string]interface{})
		if postProcessor.IsIDSelected {
			source["_id"] = doc["_id"].(string)
		}
		for fieldName, columnName := range postProcessor.SelectedFields {
			document[fieldName] = source[columnName]
		}
		documents[i] = document
	}
	rowSet := &schema.RowSet{
		Aggregates: schema.RowSetAggregates{},
	}
	if postProcessor.IsFields {
		rowSet.Rows = documents
	}

	if postProcessor.StarAggregates != "" {
		rowSet.Aggregates = schema.RowSetAggregates{
			postProcessor.StarAggregates: int(total),
		}
	}

	// Add aggregates
	if len(postProcessor.ColumnAggregate) != 0 {
		if aggregation, ok := res["aggregations"].(map[string]interface{}); ok {
			for _, column := range postProcessor.ColumnAggregate {
				if record, ok := aggregation[column].(map[string]interface{}); ok {
					val, ok := record["value"]
					if ok {
						rowSet.Aggregates[column] = val
					} else if val, ok := record["doc_count"]; ok {
						rowSet.Aggregates[column] = val
					} else {
						rowSet.Aggregates[column] = record
					}
				}
			}
		}
	}

	return rowSet
}
