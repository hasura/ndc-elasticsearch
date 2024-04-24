package connector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

func (c *Connector) Query(ctx context.Context, configuration *types.Configuration, state *types.State, request *schema.QueryRequest) (schema.QueryResponse, error) {
	rowSet, err := executeQuery(ctx, state, request)
	if err != nil {
		return nil, err
	}
	rowSets := []schema.RowSet{
		*rowSet,
	}
	return rowSets, nil
}

func executeQuery(ctx context.Context, state *types.State, request *schema.QueryRequest) (*schema.RowSet, error) {
	// Set the postProcessor in ctx
	ctx = context.WithValue(ctx, "postProcessor", &types.PostProcessor{})

	body, err := prepareElasticsearchQuery(ctx, request, state)
	if err != nil {
		return nil, err
	}

	res, err := state.Client.Search(ctx, request.Collection, body)
	if err != nil {
		return nil, err
	}

	result := prepareResponse(ctx, res)
	return result, nil
}

func prepareElasticsearchQuery(ctx context.Context, request *schema.QueryRequest, state *types.State) (map[string]interface{}, error) {
	query := map[string]interface{}{
		"_source": map[string]interface{}{
			"excludes": []string{"*"},
		},
	}

	postProcessor := ctx.Value("postProcessor").(*types.PostProcessor)

	// Select the fields
	if request.Query.Fields != nil {
		postProcessor.IsFields = true
		fields := make([]string, 0)
		selectFields := make(map[string]string)
		for fieldName, fieldData := range request.Query.Fields {
			if columnName, ok := fieldData["column"].(string); ok {
				if _, ok := state.UnsupportedQueryFields[columnName]; ok {
					return nil, schema.BadRequestError("query selection not supported on this field", map[string]interface{}{
						"value": columnName,
					})
				}
				fields = append(fields, columnName)
				selectFields[fieldName] = columnName
				if columnName == "_id" {
					postProcessor.IsIDSelected = true
				}
			}
		}
		postProcessor.SelectedFields = selectFields
		query["_source"] = fields
	}

	// Set the limit
	if request.Query.Limit != nil {
		query["size"] = *request.Query.Limit
	}

	// Set the offset
	if request.Query.Offset != nil {
		query["from"] = *request.Query.Offset
	}

	// Order by
	if request.Query.OrderBy != nil && len(request.Query.OrderBy.Elements) != 0 {
		sort, err := prepareSortQuery(request.Query.OrderBy, state)
		if err != nil {
			return nil, err
		}
		query["sort"] = sort
	}

	// Aggregations
	if request.Query.Aggregates != nil {
		aggs, err := prepareAggregateQuery(ctx, request.Query.Aggregates)
		if err != nil {
			return nil, err
		}
		if len(aggs) != 0 {
			query["aggs"] = aggs
		}
	}

	// Filter
	if request.Query.Predicate != nil {
		filter, err := prepareFilterQuery(request.Query.Predicate)
		if err != nil {
			return nil, err
		}
		if len(filter) != 0 {
			query["query"] = filter
		}
	}

	// Pretty print query
	queryJSON, _ := json.MarshalIndent(query, "", "  ")
	fmt.Println(string(queryJSON))

	return query, nil
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
	fmt.Println("Column count: ", postProcessor.ColumnCount)
	if len(postProcessor.ColumnCount) != 0 {
		for _, column := range postProcessor.ColumnCount {
			if aggregation, ok := res["aggregations"].(map[string]interface{}); ok {
				if record, ok := aggregation[column].(map[string]interface{}); ok {
					rowSet.Aggregates[column] = record["value"]
				}
			}
		}
	}

	// Pretty print res
	queryJSON, _ := json.MarshalIndent(res["aggregations"], "", "  ")
	fmt.Println(string(queryJSON))

	return rowSet
}
