package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// GetSchema returns the schema by parsing the configuration.
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

// parseConfigurationToSchema parses the given configuration to generate the schema response.
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
		properties, ok := mapping["properties"].(map[string]interface{})
		if !ok {
			continue
		}

		fields, objects := getScalarTypesAndObjects(properties, state)
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

			fieldData, ok := fieldMap["fielddata"].(bool)
			if ok {
				checkForUnsupportedFields(fieldName, fieldType, fieldData, state)
			} else {
				checkForUnsupportedFields(fieldName, fieldType, false, state)
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

						fieldData, ok := subFieldMap["fielddata"].(bool)
						if ok {
							checkForUnsupportedFields(fieldName, subFieldType, fieldData, state)
						} else {
							checkForUnsupportedFields(fieldName, subFieldType, false, state)
						}
					}
				}
			}
		} else if nestedObject, ok := fieldMap["properties"].(map[string]interface{}); ok {

			fields = append(fields, map[string]interface{}{
				"name": fieldName,
				"type": fieldName,
			})

			state.UnsupportedSortFields[fieldName] = true
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

// checkForUnsupportedFields checks for unsupported fields based on field type and field data enabled status.
func checkForUnsupportedFields(fieldName string, fieldType string, fieldDataEnalbed bool, state *types.State) {
	for _, unsupportedType := range unSupportedAggregateTypes {
		if fieldType == unsupportedType && !fieldDataEnalbed {
			state.UnsupportedAggregateFields[fieldName] = true
		}
	}
	for _, unsupportedType := range unsupportedSortDataTypes {
		if fieldType == unsupportedType && !fieldDataEnalbed {
			state.UnsupportedSortFields[fieldName] = true
		}
	}
}
