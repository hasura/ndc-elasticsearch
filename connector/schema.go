package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
	"github.com/hasura/ndc-elasticsearch/internal"
)

// GetSchema returns the schema by parsing the configuration.
func (c *Connector) GetSchema(ctx context.Context, configuration *types.Configuration, state *types.State) (schema.SchemaResponseMarshaler, error) {
	return state.Schema, nil
}

// parseConfigurationToSchema parses the given configuration to generate the schema response.
func ParseConfigurationToSchema(configuration *types.Configuration, state *types.State) *schema.SchemaResponse {
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
			Name:                  indexName,
			Arguments:             schema.CollectionInfoArguments{},
			Type:                  indexName,
			UniquenessConstraints: schema.CollectionInfoUniquenessConstraints{},
			ForeignKeys:           schema.CollectionInfoForeignKeys{},
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
				"range_queries":      make(map[string]string),
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

		if fieldType, ok := fieldIsScalar(fieldMap); ok {
			scalarFieldType := GetFieldType(fieldMap, state, indexName, fieldWithParent)
			fields = append(fields, map[string]interface{}{
				"name": fieldName,
				"type": scalarFieldType,
			})

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
	ndcSchema.ScalarTypes["_id"] = internal.ScalarTypeMap["_id"]

	// ADd the required scalar type to the schema
	for scalarTypeName, ScalarType := range internal.RequiredScalarTypes {
		ndcSchema.ScalarTypes[scalarTypeName] = ScalarType
	}

	// Add the required object types to the schema.
	for objectName, objectType := range internal.RequiredObjectTypes {
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
		if scalarType, ok := internal.ScalarTypeMap[fieldType]; ok {
			// Add the scalar type to the NDC schema
			ndcSchema.ScalarTypes[fieldType] = scalarType
		} else if objectType, ok := internal.ObjectTypeMap[fieldType]; ok {
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
