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
	// Set the sanitizer in ctx
	ctx = context.WithValue(ctx, "sanitizer", &Sanitizer{})

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

	sanitizer := ctx.Value("sanitizer").(*Sanitizer)

	// Select the fields
	if request.Query.Fields != nil {
		sanitizer.isFields = true
		fields := make([]string, 0)
		selectFields := make(map[string]string)
		for fieldName, fieldData := range request.Query.Fields {
			if columnName, ok := fieldData["column"].(string); ok {
				if _, ok := state.UnsupportedQueryFields[columnName]; ok {
					return nil, schema.BadRequestError("field is not queryable", map[string]interface{}{
						"value": columnName,
					})
				}
				fields = append(fields, columnName)
				selectFields[fieldName] = columnName
				if columnName == "_id" {
					sanitizer.isIDSelected = true
				}
			}
		}
		sanitizer.selectFields = selectFields
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
		expressionAnd, err := request.Query.Predicate.AsAnd()
		if err == nil {
			filter, err := prepareFilterQuery(expressionAnd.Expressions[0])
			if err != nil {
				return nil, err
			}
			if len(filter) != 0 {
				query["query"] = filter
			}
		} else {
			filter, err := prepareFilterQuery(request.Query.Predicate)
			if err != nil {
				return nil, err
			}
			if len(filter) != 0 {
				query["query"] = filter
			}
		}
	}

	// Pretty print query
	queryJSON, _ := json.MarshalIndent(query, "", "  ")
	fmt.Println(string(queryJSON))

	return query, nil
}

func prepareResponse(ctx context.Context, res map[string]interface{}) *schema.RowSet {
	sanitizer := ctx.Value("sanitizer").(*Sanitizer)
	total := res["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)
	hits := res["hits"].(map[string]interface{})["hits"].([]interface{})
	documents := make([]map[string]interface{}, len(hits))
	for i, hit := range hits {
		doc := hit.(map[string]interface{})
		row := make(map[string]interface{}, len(sanitizer.selectFields))
		source := doc["_source"].(map[string]interface{})
		if sanitizer.isIDSelected {
			source["_id"] = doc["_id"].(string)
		}
		for fieldName, columnName := range sanitizer.selectFields {
			row[fieldName] = source[columnName]
		}
		documents[i] = row
	}
	rowSet := &schema.RowSet{
		Aggregates: schema.RowSetAggregates{},
	}
	if sanitizer.isFields {
		rowSet.Rows = documents
	}

	if sanitizer.startAggregates != "" {
		rowSet.Aggregates = schema.RowSetAggregates{
			sanitizer.startAggregates: int(total),
		}
	}

	// Add aggregates
	fmt.Println("Column count: ", sanitizer.columnCount)
	if len(sanitizer.columnCount) != 0 {
		for _, column := range sanitizer.columnCount {
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

type Sanitizer struct {
	isFields        bool
	startAggregates string
	columnCount     []string
	isIDSelected    bool
	selectFields    map[string]string
}
