package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

func prepareSelectFields(ctx context.Context, fields schema.QueryFields, postProcessor *types.PostProcessor, parentField string) ([]string, map[string]types.Field, error) {
	source := make([]string, 0)
	selectedFields := make(map[string]types.Field)
	for fieldName, fieldData := range fields {
		columnData, err := fieldData.AsColumn()
		if err != nil {
			return nil, nil, schema.UnprocessableContentError("relationship has not been supported yet", map[string]interface{}{"value": fieldData})
		}
		column := columnData.Column
		field := types.Field{
			Name: column,
		}

		if parentField != "" {
			column = parentField + "." + columnData.Column
		} else {
			if columnData.Column == "_id" {
				postProcessor.IsIDSelected = true
			}
		}
		if columnData.Fields == nil {
			source = append(source, column)
		} else {
			nestedFields, nestedSelectFields, err := prepareNestedSelectField(ctx, columnData.Fields, postProcessor, column)
			if err != nil {
				return nil, nil, err
			}
			field.Fields = nestedSelectFields
			source = append(source, nestedFields...)
		}
		selectedFields[fieldName] = field
	}
	return source, selectedFields, nil
}

func prepareNestedSelectField(ctx context.Context, field schema.NestedField, postProcessor *types.PostProcessor, parentField string) ([]string, map[string]types.Field, error) {
	switch nestedField := field.Interface().(type) {
	case *schema.NestedObject:
		return prepareSelectFields(ctx, nestedField.Fields, postProcessor, parentField)
	case *schema.NestedArray:
		return prepareNestedSelectField(ctx, nestedField.Fields, postProcessor, parentField)
	default:
		return nil, nil, schema.UnprocessableContentError("invalid nested field", map[string]any{
			"value": field,
		})
	}
}
