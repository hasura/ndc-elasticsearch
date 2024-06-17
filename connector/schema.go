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

	indices := configuration.Indices

	for indexName, mappings := range indices {
		state.SupportedFilterFields[indexName] = map[string]interface{}{
			"term_level_queries": make(map[string]string),
			"unstructured_text":  make(map[string]string),
			"full_text_queries":  make(map[string]string),
		}
		state.NestedFields[indexName] = make(map[string]string)
		state.SupportedAggregateFields[indexName] = make(map[string]string)
		state.SupportedSortFields[indexName] = make(map[string]string)
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

		fields, objects := getScalarTypesAndObjects(properties, state, indexName, "")
		prepareNdcSchema(&ndcSchema, indexName, fields, objects)

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

	nativeQueries := configuration.Queries

	ndcSchema = parseNativeQueryToSchema(&ndcSchema, state, nativeQueries)

	return &ndcSchema
}

// parseNativeQueryToSchema parses the given native queries and adds them to the schema response.
// It also handles return types of kind "defination" and updates the state accordingly.
func parseNativeQueryToSchema(schemaResponse *schema.SchemaResponse, state *types.State, nativeQueries map[string]types.NativeQuery) schema.SchemaResponse {
	for queryName, queryConfig := range nativeQueries {
		indexName := queryConfig.Index

		returnType := queryConfig.ReturnType
		returnTypeKind := returnType.Kind

		if returnTypeKind == "defination" {
			indexName = queryName
			mapping := returnType.Mappings

			properties, ok := (*mapping)["properties"].(map[string]interface{})
			if !ok {
				continue
			}

			state.SupportedFilterFields[indexName] = map[string]interface{}{
				"term_level_queries": make(map[string]string),
				"unstructured_text":  make(map[string]string),
				"full_text_queries":  make(map[string]string),
			}
			state.NestedFields[indexName] = make(map[string]string)
			state.SupportedAggregateFields[indexName] = make(map[string]string)
			state.SupportedSortFields[indexName] = make(map[string]string)
			fields, objects := getScalarTypesAndObjects(properties, state, indexName, "")
			prepareNdcSchema(schemaResponse, indexName, fields, objects)
		}

		// Get arguments for the collection info
		arguments := schema.CollectionInfoArguments{}
		if queryConfig.Arguments != nil {
			arguments = getNdcArguments(*queryConfig.Arguments)
		}

		collectionInfo := schema.CollectionInfo{
			Name:      queryName,
			Arguments: arguments,
			Type:      indexName,
			UniquenessConstraints: schema.CollectionInfoUniquenessConstraints{
				queryName + "_by_id": schema.UniquenessConstraint{
					UniqueColumns: []string{"_id"},
				},
			},
			ForeignKeys: schema.CollectionInfoForeignKeys{},
		}

		schemaResponse.Collections = append(schemaResponse.Collections, collectionInfo)
	}

	return *schemaResponse
}

// getNdcArguments converts the query parameters to NDC ArgumentInfo objects.
func getNdcArguments(parameters map[string]interface{}) schema.CollectionInfoArguments {
	arguments := schema.CollectionInfoArguments{}

	for argName, argData := range parameters {
		argMap, ok := argData.(map[string]interface{})
		if !ok {
			continue
		}

		typeStr := argMap["type"].(string)
		typeObj := schema.NewNamedType(typeStr)

		arguments[argName] = schema.ArgumentInfo{
			Type: typeObj.Encode(),
		}
	}

	return arguments
}

// getScalarTypesAndObjects retrieves scalar types and objects from properties.
func getScalarTypesAndObjects(properties map[string]interface{}, state *types.State, indexName string, parentField string) ([]map[string]interface{}, []map[string]interface{}) {
	fields := make([]map[string]interface{}, 0)
	objects := make([]map[string]interface{}, 0)
	for fieldName, fieldData := range properties {
		fieldWithParent := fieldName
		if parentField != "" {
			fieldWithParent = parentField + "." + fieldName
		}
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
					state.SupportedSortFields[indexName].(map[string]string)[fieldWithParent] = fieldWithParent
				}
				if isAggregateSupported(fieldType, fieldDataEnalbed) {
					state.SupportedAggregateFields[indexName].(map[string]string)[fieldWithParent] = fieldWithParent
				}
			} else {
				if isSortSupported(fieldType, false) {
					state.SupportedSortFields[indexName].(map[string]string)[fieldWithParent] = fieldWithParent
				}
				if isAggregateSupported(fieldType, false) {
					state.SupportedAggregateFields[indexName].(map[string]string)[fieldWithParent] = fieldWithParent
				}
			}

			if subFields, ok := fieldMap["fields"].(map[string]interface{}); ok {
				for subFieldName, subFieldData := range subFields {
					subFieldMap := subFieldData.(map[string]interface{})
					if subFieldType, ok := subFieldMap["type"].(string); ok {
						subFieldWithParent := fieldWithParent + "." + subFieldName

						fieldDataEnalbed, ok := subFieldMap["fielddata"].(bool)
						if ok {
							if isSortSupported(subFieldType, fieldDataEnalbed) {
								state.SupportedSortFields[indexName].(map[string]string)[subFieldWithParent] = subFieldWithParent
								if _, ok := state.SupportedSortFields[indexName].(map[string]string)[fieldWithParent]; !ok {
									state.SupportedSortFields[indexName].(map[string]string)[fieldWithParent] = subFieldWithParent
								}
							}
							if isAggregateSupported(subFieldType, fieldDataEnalbed) {
								state.SupportedAggregateFields[indexName].(map[string]string)[subFieldWithParent] = subFieldWithParent
								if _, ok := state.SupportedAggregateFields[indexName].(map[string]string)[fieldWithParent]; !ok {
									state.SupportedAggregateFields[indexName].(map[string]string)[fieldWithParent] = subFieldWithParent
								}
							}
						} else {
							if isSortSupported(subFieldType, false) {
								state.SupportedSortFields[indexName].(map[string]string)[subFieldWithParent] = subFieldWithParent
								if _, ok := state.SupportedSortFields[indexName].(map[string]string)[fieldWithParent]; !ok {
									state.SupportedSortFields[indexName].(map[string]string)[fieldWithParent] = subFieldWithParent
								}
							}
							if isAggregateSupported(subFieldType, false) {
								state.SupportedAggregateFields[indexName].(map[string]string)[subFieldWithParent] = subFieldWithParent
								if _, ok := state.SupportedAggregateFields[indexName].(map[string]string)[fieldWithParent]; !ok {
									state.SupportedAggregateFields[indexName].(map[string]string)[fieldWithParent] = subFieldWithParent
								}
							}
						}

						if subFieldType == "keyword" {
							state.SupportedFilterFields[indexName].(map[string]interface{})["term_level_queries"].(map[string]string)[fieldWithParent] = subFieldWithParent
						} else if subFieldType == "wildcard" {
							state.SupportedFilterFields[indexName].(map[string]interface{})["unstructured_text"].(map[string]string)[fieldWithParent] = subFieldWithParent
						} else if subFieldType == "text" {
							state.SupportedFilterFields[indexName].(map[string]interface{})["full_text_queries"].(map[string]string)[fieldWithParent] = subFieldWithParent
						}
					}
				}
			}
			if fieldType == "wildcard" {
				state.SupportedFilterFields[indexName].(map[string]interface{})["unstructured_text"].(map[string]string)[fieldWithParent] = fieldWithParent
			}
			if fieldType == "keyword" {
				state.SupportedFilterFields[indexName].(map[string]interface{})["term_level_queries"].(map[string]string)[fieldWithParent] = fieldWithParent
			}
		} else if nestedObject, ok := fieldMap["properties"].(map[string]interface{}); ok {
			if fieldType == "nested" {
				state.NestedFields[indexName].(map[string]string)[fieldWithParent] = fieldType
			}
			fields = append(fields, map[string]interface{}{
				"name": fieldName,
				"type": fieldName,
				"obj":  true,
			})

			flds, objs := getScalarTypesAndObjects(nestedObject, state, indexName, fieldWithParent)
			objects = append(objects, map[string]interface{}{
				"name":   fieldName,
				"fields": flds,
			})
			objects = append(objects, objs...)
		}
	}
	return fields, objects
}

// prepareNdcSchema prepares the NDC schema.
func prepareNdcSchema(ndcSchema *schema.SchemaResponse, index string, fields []map[string]interface{}, objects []map[string]interface{}) {

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

// isSortSupported checks if a field type is supported for sorting
// based on fielddata and unsupported sort data types.
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

// isAggregateSupported checks if a field type is supported for aggregation
// based on fielddata and unsupported aggregate data types.
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
