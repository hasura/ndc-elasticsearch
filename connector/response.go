package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// prepareResponseWithVariables prepares a row set for query with variables based on the elastic search response data.
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

		rowSet.Aggregates = extractAggregates(rowSet.Aggregates, bucketData, postProcessor)
		rowSets = append(rowSets, *rowSet)
	}
	return rowSets
}

// prepareResponse prepares a row set based on elastic search response data.
func prepareResponse(ctx context.Context, response map[string]interface{}) *schema.RowSet {
	postProcessor := ctx.Value("postProcessor").(*types.PostProcessor)

	totalHits := response["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)
	hits := response["hits"].(map[string]interface{})["hits"].([]interface{})

	documents := make([]map[string]interface{}, len(hits))
	for i, hit := range hits {
		document := extractDocument(hit, postProcessor)
		documents[i] = document
	}

	rowSet := &schema.RowSet{
		Aggregates: schema.RowSetAggregates{},
	}
	aggregations, ok := response["aggregations"].(map[string]interface{})
	if ok {
		rowSet.Aggregates = extractAggregates(rowSet.Aggregates, aggregations, postProcessor)
	}

	if postProcessor.IsFields {
		rowSet.Rows = documents
	}

	if postProcessor.StarAggregates != "" {
		rowSet.Aggregates[postProcessor.StarAggregates] = int(totalHits)
	}

	return rowSet
}

// extractDocument extracts document fields based on the selected fields from the source data.
func extractDocument(hit interface{}, postProcessor *types.PostProcessor) map[string]interface{} {
	hitMap := hit.(map[string]interface{})
	source := hitMap["_source"].(map[string]interface{})

	if postProcessor.IsIDSelected {
		source["_id"] = hitMap["_id"].(string)
	}

	document := make(map[string]interface{}, len(postProcessor.SelectedFields))
	for fieldName, columnName := range postProcessor.SelectedFields {
		document[fieldName] = source[columnName]
	}

	return document
}

// extractAggregates extracts aggregate values from the source data and updates the row set aggregates.
func extractAggregates(aggregates schema.RowSetAggregates, aggregations map[string]interface{}, postProcessor *types.PostProcessor) schema.RowSetAggregates {
	if len(postProcessor.ColumnAggregate) == 0 {
		return aggregates
	}

	for _, column := range postProcessor.ColumnAggregate {
		if record, ok := aggregations[column].(map[string]interface{}); ok {
			val, ok := record["value"]
			if ok {
				aggregates[column] = val
			} else if val, ok := record["doc_count"]; ok {
				aggregates[column] = val
			} else {
				aggregates[column] = record
			}
		}
	}

	return aggregates
}
