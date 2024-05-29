package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// GetSchema returns the schema by parsing the configuration.
func (c *Connector) GetSchema(ctx context.Context, configuration *types.Configuration, state *types.State) (schema.SchemaResponseMarshaler, error) {
	return state.Schema, nil
}

// parseConfigurationToSchema parses the given configuration to generate the schema response.
func parseConfigurationToSchema(configuration *types.Configuration, state *types.State) *schema.SchemaResponse {
	ndcSchema := schema.SchemaResponse{
		ScalarTypes: make(schema.SchemaResponseScalarTypes),
		ObjectTypes: make(schema.SchemaResponseObjectTypes),
		Collections: []schema.CollectionInfo{},
		Functions:   []schema.FunctionInfo{},
		Procedures:  []schema.ProcedureInfo{},
	}

	for indexName, mappings := range *configuration {
		state.SupportedFilterFields[indexName] = map[string]interface{}{
			"term_level_queries": make(map[string]string),
			"unstructured_text":  make(map[string]string),
			"full_text_queries":  make(map[string]string),
		}
		state.SupportedSortFields[indexName] = make(map[string]string)
		state.SupportedAggregateFields[indexName] = make(map[string]string)
		data, ok := mappings.(map[string]interface{})
		if !ok {
			continue
		}
		mapping, ok := data["mappings"].(map[string]interface{})
		if !ok {
			continue
		}
		properties, ok := mapping["properties"].(map[string]interface{})
		if !ok {
			continue
		}

		fields, objects := getScalarTypesAndObjects(properties, state, indexName)
		prepareNDCSchema(&ndcSchema, indexName, fields, objects)

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
	return &ndcSchema
}

// getScalarTypesAndObjects retrieves scalar types and objects from properties.
func getScalarTypesAndObjects(properties map[string]interface{}, state *types.State, indexName string) ([]map[string]interface{}, []map[string]interface{}) {
	fields := make([]map[string]interface{}, 0)
	objects := make([]map[string]interface{}, 0)
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
			if fieldType == "aggregate_metric_double" {
				metrics, ok := fieldMap["metrics"].([]interface{})
				metricFields := schema.ObjectTypeFields{}
				if ok {
					for _, metric := range metrics {
						metricFields[metric.(string)] = schema.ObjectField{Type: schema.NewNamedType("double").Encode()}
					}
					objectTypeMap[fieldType] = schema.ObjectType{
						Fields: metricFields,
					}
				}
			}

			fieldDataEnalbed, ok := fieldMap["fielddata"].(bool)
			if ok {
				if isSortSupported(fieldType, fieldDataEnalbed) {
					state.SupportedSortFields[indexName].(map[string]string)[fieldName] = fieldName
				}
				if isAggregateSupported(fieldType, fieldDataEnalbed) {
					state.SupportedAggregateFields[indexName].(map[string]string)[fieldName] = fieldName
				}
			} else {
				if isSortSupported(fieldType, false) {
					state.SupportedSortFields[indexName].(map[string]string)[fieldName] = fieldName
				}
				if isAggregateSupported(fieldType, false) {
					state.SupportedAggregateFields[indexName].(map[string]string)[fieldName] = fieldName
				}
			}

			if subFields, ok := fieldMap["fields"].(map[string]interface{}); ok {
				for subFieldName, subFieldData := range subFields {
					subFieldMap := subFieldData.(map[string]interface{})
					if subFieldType, ok := subFieldMap["type"].(string); ok {
						name := fieldName + "." + subFieldName

						fieldDataEnalbed, ok := subFieldMap["fielddata"].(bool)
						if ok {
							if isSortSupported(subFieldType, fieldDataEnalbed) {
								state.SupportedSortFields[indexName].(map[string]string)[name] = name
								if _, ok := state.SupportedSortFields[indexName].(map[string]string)[fieldName]; !ok {
									state.SupportedSortFields[indexName].(map[string]string)[fieldName] = name
								}
							}
							if isAggregateSupported(subFieldType, fieldDataEnalbed) {
								state.SupportedAggregateFields[indexName].(map[string]string)[name] = name
								if _, ok := state.SupportedAggregateFields[indexName].(map[string]string)[fieldName]; !ok {
									state.SupportedAggregateFields[indexName].(map[string]string)[fieldName] = name
								}
							}
						} else {
							if isSortSupported(subFieldType, false) {
								state.SupportedSortFields[indexName].(map[string]string)[name] = name
								if _, ok := state.SupportedSortFields[indexName].(map[string]string)[fieldName]; !ok {
									state.SupportedSortFields[indexName].(map[string]string)[fieldName] = name
								}
							}
							if isAggregateSupported(subFieldType, false) {
								state.SupportedAggregateFields[indexName].(map[string]string)[name] = name
								if _, ok := state.SupportedAggregateFields[indexName].(map[string]string)[fieldName]; !ok {
									state.SupportedAggregateFields[indexName].(map[string]string)[fieldName] = name
								}
							}
						}

						if subFieldType == "keyword" {
							state.SupportedFilterFields[indexName].(map[string]interface{})["term_level_queries"].(map[string]string)[fieldName] = name
						} else if subFieldType == "wildcard" {
							state.SupportedFilterFields[indexName].(map[string]interface{})["unstructured_text"].(map[string]string)[fieldName] = name
						} else if subFieldType == "text" {
							state.SupportedFilterFields[indexName].(map[string]interface{})["full_text_queries"].(map[string]string)[fieldName] = name
						}
					}
				}
			}
			if fieldType == "wildcard" {
				state.SupportedFilterFields[indexName].(map[string]interface{})["unstructured_text"].(map[string]string)[fieldName] = fieldName
			}
			if fieldType == "keyword" {
				state.SupportedFilterFields[indexName].(map[string]interface{})["term_level_queries"].(map[string]string)[fieldName] = fieldName
			}
		} else if nestedObject, ok := fieldMap["properties"].(map[string]interface{}); ok {
			fields = append(fields, map[string]interface{}{
				"name": fieldName,
				"type": fieldName,
				"obj":  true,
			})

			flds, objs := getScalarTypesAndObjects(nestedObject, state, indexName)
			objects = append(objects, map[string]interface{}{
				"name":   fieldName,
				"fields": flds,
			})
			objects = append(objects, objs...)
		}
	}
	return fields, objects
}

// prepareNDCSchema prepares the NDC schema.
func prepareNDCSchema(ndcSchema *schema.SchemaResponse, index string, fields []map[string]interface{}, objects []map[string]interface{}) {

	collectionFields := make(schema.ObjectTypeFields)
	for _, field := range fields {
		fieldType := field["type"].(string)
		fieldName := field["name"].(string)
		if scalarType, ok := scalarTypeMap[fieldType]; ok {
			ndcSchema.ScalarTypes[fieldType] = scalarType
		}
		if _, ok := field["obj"]; ok {
			collectionFields[fieldName] = schema.ObjectField{
				Type: schema.NewArrayType(schema.NewNamedType(fieldType)).Encode(),
			}
		} else {
			collectionFields[fieldName] = schema.ObjectField{
				Type: schema.NewNamedType(fieldType).Encode(),
			}
		}

		if objectType, ok := objectTypeMap[fieldType]; ok {
			ndcSchema.ObjectTypes[fieldType] = objectType
		}
	}

	collectionFields["_id"] = schema.ObjectField{
		Type: schema.NewNamedType("_id").Encode(),
	}
	ndcSchema.ScalarTypes["_id"] = scalarTypeMap["_id"]
	ndcSchema.ScalarTypes["double"] = scalarTypeMap["double"]
	ndcSchema.ScalarTypes["integer"] = scalarTypeMap["integer"]

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

			if _, ok := field["obj"]; ok {
				ndcObjectFields[fieldName] = schema.ObjectField{
					Type: schema.NewArrayType(schema.NewNamedType(fieldType)).Encode(),
				}
			} else {
				ndcObjectFields[fieldName] = schema.ObjectField{
					Type: schema.NewNamedType(fieldType).Encode(),
				}
			}

			if objectType, ok := objectTypeMap[fieldType]; ok {
				ndcSchema.ObjectTypes[fieldType] = objectType
			}
		}
		ndcSchema.ObjectTypes[objectName] = schema.ObjectType{
			Fields: ndcObjectFields,
		}
	}
	// Iterate throgh all static object types
	for objectName, objectType := range objectTypeMap {
		ndcSchema.ObjectTypes[objectName] = objectType
	}
}

func isSortSupported(fieldType string, fieldDataEnalbed bool) bool {
	if fieldDataEnalbed {
		return true
	}
	for _, unSupportedType := range unsupportedSortDataTypes {
		if fieldType == unSupportedType {
			return false
		}
	}
	return true
}
func isAggregateSupported(fieldType string, fieldDataEnalbed bool) bool {
	if fieldDataEnalbed {
		return true
	}
	for _, unSupportedType := range unSupportedAggregateTypes {
		if fieldType == unSupportedType {
			return false
		}
	}
	return true
}
