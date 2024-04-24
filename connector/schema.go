package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

func (c *Connector) GetSchema(ctx context.Context, configuration *types.Configuration, state *types.State) (schema.SchemaResponseMarshaler, error) {
	schemaObject := schema.SchemaResponse{
		ScalarTypes: make(schema.SchemaResponseScalarTypes),
		ObjectTypes: make(schema.SchemaResponseObjectTypes),
		Collections: []schema.CollectionInfo{},
		Functions:   []schema.FunctionInfo{},
		Procedures:  []schema.ProcedureInfo{},
	}

	parseConfigurationToSchema(configuration, &schemaObject, state)
	return schemaObject, nil
}

func parseConfigurationToSchema(configuration *types.Configuration, ndcSchema *schema.SchemaResponse, state *types.State) {

	for indexName, mappings := range *configuration {
		data, ok := mappings.(map[string]interface{})
		if !ok {
			continue
		}
		mapping, ok := data["mappings"].(map[string]interface{})
		if !ok {
			continue
		}
		fields, objects := getScalarTypesAndObjects(mapping, state)
		prepareNDCSchema(ndcSchema, indexName, fields, objects)

		ndcSchema.Collections = append(ndcSchema.Collections, schema.CollectionInfo{
			Name:      indexName,
			Arguments: schema.CollectionInfoArguments{},
			Type:      indexName,
			UniquenessConstraints: schema.CollectionInfoUniquenessConstraints{
				indexName + "_by_id": schema.UniquenessConstraint{
					UniqueColumns: []string{"_id"},
				},
			},
			ForeignKeys: schema.CollectionInfoForeignKeys{},
		})
	}
}

func getScalarTypesAndObjects(data map[string]interface{}, state *types.State) ([]map[string]interface{}, []map[string]interface{}) {
	fields := make([]map[string]interface{}, 0)
	objects := make([]map[string]interface{}, 0)
	if properties, ok := data["properties"].(map[string]interface{}); ok {
		for fieldName, fieldData := range properties {
			fieldMap, ok := fieldData.(map[string]interface{})
			if !ok {
				continue
			}
			if fieldType, ok := fieldMap["type"].(string); ok && fieldType != "nested" && fieldType != "object" && fieldType != "flattened" {
				fields = append(fields, map[string]interface{}{
					"name": fieldName,
					"type": fieldType,
				})

				for _, unsupportedSortDataType := range unsupportedSortDataTypes {
					fieldData, ok := fieldMap["fielddata"].(bool)
					if ok {
						if fieldType == unsupportedSortDataType && !fieldData {
							state.UnsupportedSortFields = append(state.UnsupportedSortFields, fieldName)
						}
					} else {
						if fieldType == unsupportedSortDataType {
							state.UnsupportedSortFields = append(state.UnsupportedSortFields, fieldName)
						}
					}
				}

				if subFields, ok := fieldMap["fields"].(map[string]interface{}); ok {
					for subFieldName, subFieldData := range subFields {
						subFieldMap := subFieldData.(map[string]interface{})
						if subFieldType, ok := subFieldMap["type"].(string); ok {
							name := fieldName + "." + subFieldName
							subField := map[string]interface{}{
								"name": name,
								"type": subFieldType,
							}
							state.UnsupportedQueryFields[name] = fieldName
							fields = append(fields, subField)

							for _, unsupportedSortDataType := range unsupportedSortDataTypes {
								fieldData, ok := subFieldMap["fielddata"].(bool)
								if ok {
									if subFieldType == unsupportedSortDataType && !fieldData {
										state.UnsupportedSortFields = append(state.UnsupportedSortFields, name)
									}
								} else {
									if subFieldType == unsupportedSortDataType {
										state.UnsupportedSortFields = append(state.UnsupportedSortFields, name)
									}
								}
							}
						}
					}
				}
			} else if _, ok := fieldMap["properties"].(map[string]interface{}); ok {

				fields = append(fields, map[string]interface{}{
					"name": fieldName,
					"type": fieldName,
				})

				state.UnsupportedSortFields = append(state.UnsupportedSortFields, fieldName)
				flds, objs := getScalarTypesAndObjects(fieldMap, state)
				objects = append(objects, map[string]interface{}{
					"name":   fieldName,
					"fields": flds,
				})
				objects = append(objects, objs...)
			}
		}
	}
	return fields, objects
}

func prepareNDCSchema(ndcSchema *schema.SchemaResponse, index string, fields []map[string]interface{}, objects []map[string]interface{}) {

	collectionFields := make(schema.ObjectTypeFields)
	for _, field := range fields {
		fieldType := field["type"].(string)
		fieldName := field["name"].(string)
		if scalarType, ok := scalarTypeMap[fieldType]; ok {
			ndcSchema.ScalarTypes[fieldType] = scalarType
		}
		collectionFields[fieldName] = schema.ObjectField{
			Type: schema.NewNamedType(fieldType).Encode(),
		}

		if objectType, ok := objectTypeMap[fieldType]; ok {
			ndcSchema.ObjectTypes[fieldType] = objectType
		}
	}

	collectionFields["_id"] = schema.ObjectField{
		Type: schema.NewNamedType("_id").Encode(),
	}
	ndcSchema.ScalarTypes["_id"] = scalarTypeMap["_id"]

	ndcSchema.ObjectTypes[index] = schema.ObjectType{
		Fields: collectionFields,
	}

	for _, object := range objects {
		ndcObjectFields := make(schema.ObjectTypeFields)
		objectName := object["name"].(string)
		objectFields := object["fields"].([]map[string]interface{})

		for _, field := range objectFields {
			fieldType := field["type"].(string)
			fieldName := field["name"].(string)
			if scalarType, ok := scalarTypeMap[fieldType]; ok {
				ndcSchema.ScalarTypes[fieldType] = scalarType
			}

			ndcObjectFields[fieldName] = schema.ObjectField{
				Type: schema.NewNamedType(fieldType).Encode(),
			}

			if objectType, ok := objectTypeMap[fieldType]; ok {
				ndcSchema.ObjectTypes[fieldType] = objectType
			}
		}
		ndcSchema.ObjectTypes[objectName] = schema.ObjectType{
			Fields: ndcObjectFields,
		}
	}
}
