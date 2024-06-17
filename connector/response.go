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
		doc := hit.(map[string]interface{})
		source := doc["_source"].(map[string]interface{})
		if postProcessor.IsIDSelected {
			source["_id"] = doc["_id"].(string)
		}
		documents[i] = extractDocument(source, postProcessor.SelectedFields)
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

// extractDocument extracts selected fields from the source data.
func extractDocument(source map[string]interface{}, selectedFields map[string]types.Field) map[string]interface{} {
	document := make(map[string]interface{})
	for fieldName, fieldData := range selectedFields {
		document[fieldName] = extractSubDocument(source, fieldData)
	}
	return document
}

// extractSubDocument extracts sub-documents based on the selected fields.
func extractSubDocument(source map[string]interface{}, fieldData types.Field) interface{} {
	if fieldData.Fields != nil {
		sourceData, ok := source[fieldData.Name]
		if !ok {
			return []interface{}{extractDocument(make(map[string]interface{}), fieldData.Fields)}
		}

		if subDocument, ok := sourceData.(map[string]interface{}); ok {
			return []interface{}{extractDocument(subDocument, fieldData.Fields)}
		}

		if subDocuments, ok := sourceData.([]interface{}); ok {
			subDocumentsList := make([]interface{}, 0)
			for _, subDocument := range subDocuments {
				subDocumentsList = append(subDocumentsList, extractDocument(subDocument.(map[string]interface{}), fieldData.Fields))
			}
			return subDocumentsList
		}
	}

	if fieldValue, ok := source[fieldData.Name]; ok {
		return fieldValue
	}

	return nil
}

// extractAggregates extracts aggregate values from the source data and updates the row set aggregates.
func extractAggregates(aggregates schema.RowSetAggregates, aggregations map[string]interface{}, postProcessor *types.PostProcessor) schema.RowSetAggregates {
	if len(postProcessor.ColumnAggregate) == 0 {
		return aggregates
	}

	for aggName, isNested := range postProcessor.ColumnAggregate {
		if record, ok := aggregations[aggName].(map[string]interface{}); ok {
			if isNested {
				record = record[aggName].(map[string]interface{})
			}
			val, ok := record["value"]
			if ok {
				aggregates[aggName] = val
			} else if val, ok := record["doc_count"]; ok {
				aggregates[aggName] = val
			} else {
				aggregates[aggName] = record
			}
		}
	}

	return aggregates
}
