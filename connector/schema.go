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

	state.SupportedFilterFields["term_level_queries"] = map[string]string{}
	state.SupportedFilterFields["unstructured_text"] = map[string]string{}
	state.SupportedFilterFields["full_text_queries"] = map[string]string{}
	for indexName, mappings := range *configuration {
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

		fields, objects := getScalarTypesAndObjects(properties, state)
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
func getScalarTypesAndObjects(properties map[string]interface{}, state *types.State) ([]map[string]interface{}, []map[string]interface{}) {
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
					state.SupportedSortFields[fieldName] = fieldName
				}
				if isAggregateSupported(fieldType, fieldDataEnalbed) {
					state.SupportedAggregateFields[fieldName] = fieldName
				}
			} else {
				if isSortSupported(fieldType, false) {
					state.SupportedSortFields[fieldName] = fieldName
				}
				if isAggregateSupported(fieldType, false) {
					state.SupportedAggregateFields[fieldName] = fieldName
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
								state.SupportedSortFields[name] = name
								if _, ok := state.SupportedSortFields[fieldName]; !ok {
									state.SupportedSortFields[fieldName] = name
								}
							}
							if isAggregateSupported(subFieldType, fieldDataEnalbed) {
								state.SupportedAggregateFields[name] = name
								if _, ok := state.SupportedAggregateFields[fieldName]; !ok {
									state.SupportedAggregateFields[fieldName] = name
								}
							}
						} else {
							if isSortSupported(subFieldType, false) {
								state.SupportedSortFields[name] = name
								if _, ok := state.SupportedSortFields[fieldName]; !ok {
									state.SupportedSortFields[fieldName] = name
								}
							}
							if isAggregateSupported(subFieldType, false) {
								state.SupportedAggregateFields[name] = name
								if _, ok := state.SupportedAggregateFields[fieldName]; !ok {
									state.SupportedAggregateFields[fieldName] = name
								}
							}
						}

						if subFieldType == "keyword" {
							state.SupportedFilterFields["term_level_queries"].(map[string]string)[fieldName] = name
						} else if subFieldType == "wildcard" {
							state.SupportedFilterFields["unstructured_text"].(map[string]string)[fieldName] = name
						} else if subFieldType == "text" {
							state.SupportedFilterFields["full_text_queries"].(map[string]string)[fieldName] = name
						}
					}
				}
			}
		} else if nestedObject, ok := fieldMap["properties"].(map[string]interface{}); ok {

			fields = append(fields, map[string]interface{}{
				"name": fieldName,
				"type": fieldName,
			})

			flds, objs := getScalarTypesAndObjects(nestedObject, state)
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
