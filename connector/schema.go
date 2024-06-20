package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/internal"
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
			"range_queries":      make(map[string]string),
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
	return &ndcSchema
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
						if metricValue, ok := metric.(string); ok {
							metricFields[metricValue] = schema.ObjectField{Type: schema.NewNamedType("double").Encode()}
						}
					}
					objectTypeMap[fieldType] = schema.ObjectType{
						Fields: metricFields,
					}
				}
			}
			fieldDataEnalbed := false
			if fieldData, ok := fieldMap["fielddata"].(bool); ok {
				fieldDataEnalbed = fieldData
			}
			if isSortSupported(fieldType, fieldDataEnalbed) {
				state.SupportedSortFields[indexName].(map[string]string)[fieldWithParent] = fieldWithParent
			}
			if isAggregateSupported(fieldType, fieldDataEnalbed) {
				state.SupportedAggregateFields[indexName].(map[string]string)[fieldWithParent] = fieldWithParent
			}

			if subFields, ok := fieldMap["fields"].(map[string]interface{}); ok {
				handleSubFields(state, subFields, indexName, fieldWithParent)
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

// handleSubFields processes the subfields of a parent field and updates the state
// accordingly.
func handleSubFields(state *types.State, subFields map[string]interface{}, indexName string, parentField string) {
	for subFieldName, subFieldData := range subFields {
		subFieldMap, ok := subFieldData.(map[string]interface{})
		if !ok {
			continue
		}
		subFieldType, ok := subFieldMap["type"].(string)
		if !ok {
			continue
		}

		fieldDataEnalbed := false
		if fieldData, ok := subFieldMap["fielddata"].(bool); ok {
			fieldDataEnalbed = fieldData
		}
		subFieldWithParent := parentField + "." + subFieldName

		// Update the supported sort fields if the subfield is sortable.
		if isSortSupported(subFieldType, fieldDataEnalbed) {
			state.SupportedSortFields[indexName].(map[string]string)[subFieldWithParent] = subFieldWithParent
			if _, ok := state.SupportedSortFields[indexName].(map[string]string)[parentField]; !ok {
				state.SupportedSortFields[indexName].(map[string]string)[parentField] = subFieldWithParent
			}
		}

		// Update the supported aggregate fields if the subfield is aggregatable.
		if isAggregateSupported(subFieldType, fieldDataEnalbed) {
			state.SupportedAggregateFields[indexName].(map[string]string)[subFieldWithParent] = subFieldWithParent
			if _, ok := state.SupportedAggregateFields[indexName].(map[string]string)[parentField]; !ok {
				state.SupportedAggregateFields[indexName].(map[string]string)[parentField] = subFieldWithParent
			}
		}

		// Update the supported filter fields based on the subfield type.
		if subFieldType == "keyword" {
			state.SupportedFilterFields[indexName].(map[string]interface{})["term_level_queries"].(map[string]string)[parentField] = subFieldWithParent
		} else if subFieldType == "wildcard" {
			state.SupportedFilterFields[indexName].(map[string]interface{})["unstructured_text"].(map[string]string)[parentField] = subFieldWithParent
		} else if subFieldType == "text" {
			state.SupportedFilterFields[indexName].(map[string]interface{})["full_text_queries"].(map[string]string)[parentField] = subFieldWithParent
		} else if internal.Contains(numericFields, subFieldType) {
			state.SupportedFilterFields[indexName].(map[string]interface{})["range_queries"].(map[string]string)[parentField] = subFieldWithParent
		}
	}
}

// prepareNdcSchema prepares the NDC schema. It takes in the NDC schema,
// the index name, the fields and objects from Elasticsearch mappings,
// and adds them to the NDC schema.
func prepareNdcSchema(ndcSchema *schema.SchemaResponse, index string, fields []map[string]interface{}, objects []map[string]interface{}) {
	// Get the object fields for Elasticsearch index
	collectionFields := getNdcObjectFields(fields, ndcSchema)

	// Add the _id field and its type to the schema. This field will not be fetched from Elasticsearch mappings.
	collectionFields["_id"] = schema.ObjectField{
		Type: schema.NewNamedType("_id").Encode(),
	}

	// Add the object type for the index to the schema.
	ndcSchema.ObjectTypes[index] = schema.ObjectType{
		Fields: collectionFields,
	}

	// Add the object types for the objects from Elasticsearch mappings to the schema.
	for _, object := range objects {
		objectName := object["name"].(string)
		objectFields := object["fields"].([]map[string]interface{})
		ndcObjectFields := getNdcObjectFields(objectFields, ndcSchema)

		ndcSchema.ObjectTypes[objectName] = schema.ObjectType{
			Fields: ndcObjectFields,
		}
	}

	// Add the required fields to the schema
	ndcSchema.ScalarTypes["_id"] = scalarTypeMap["_id"]

	// ADd the required scalar type to the schema
	for scalarTypeName, ScalarType := range requiredScalarTypes {
		ndcSchema.ScalarTypes[scalarTypeName] = ScalarType
	}

	// Add the required object types to the schema.
	for objectName, objectType := range requiredObjectTypes {
		ndcSchema.ObjectTypes[objectName] = objectType
	}
}

// getNdcObjectFields generates the object fields for the NDC schema
// based on the Elasticsearch fields.
func getNdcObjectFields(fields []map[string]interface{}, ndcSchema *schema.SchemaResponse) schema.ObjectTypeFields {
	// Initialize the object fields for the NDC schema
	ndcObjectFields := make(schema.ObjectTypeFields)

	// Iterate through each field in the Elasticsearch fields
	for _, field := range fields {
		fieldType := field["type"].(string)
		fieldName := field["name"].(string)

		// Add scalar or object type to the schema
		if scalarType, ok := scalarTypeMap[fieldType]; ok {
			// Add the scalar type to the NDC schema
			ndcSchema.ScalarTypes[fieldType] = scalarType
		} else if objectType, ok := objectTypeMap[fieldType]; ok {
			// Add the object type to the NDC schema
			ndcSchema.ObjectTypes[fieldType] = objectType
		}

		// Check if the field is of type object or nested in Elasticsearch
		if _, ok := field["obj"]; ok {
			// If it is nested, make it an array in the schema
			ndcObjectFields[fieldName] = schema.ObjectField{
				Type: schema.NewArrayType(schema.NewNamedType(fieldType)).Encode(),
			}
		} else {
			// If it is not nested, make it an object in the schema
			ndcObjectFields[fieldName] = schema.ObjectField{
				Type: schema.NewNamedType(fieldType).Encode(),
			}
		}
	}

	// Return the object fields for the NDC schema
	return ndcObjectFields
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
