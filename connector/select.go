package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

func prepareSelectQuery(ctx context.Context, state *types.State, ndcFields schema.QueryFields) ([]string, error) {
	postProcessor := ctx.Value("postProcessor").(*types.PostProcessor)
	postProcessor.IsFields = true
	fields := make([]string, 0)
	selectFields := make(map[string]string)
	for fieldName, fieldData := range ndcFields {
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
	return fields, nil
}
