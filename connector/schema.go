package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// collectionObjects holds the intermediate (pre-emission) field/object lists
// produced for a single collection (an index or a native query definition).
// We collect these for every collection first and then emit every nested object
// type under a fully-qualified `index.path.to.field` name — this is what
// prevents the object-type name-collision bug (ENT-82) where identically-named
// objects in different collections (or inner/outer objects sharing a name)
// overwrote each other in the single global ObjectTypes map, silently dropping
// fields.
type collectionObjects struct {
	name    string                   // collection / index-level object-type name
	fields  []map[string]interface{} // top-level fields of the collection
	objects []map[string]interface{} // nested object types (flattened)
}

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

	// Phase 1: walk every index and collect its fields/objects. Nested object
	// types are named with a fully-qualified `index.path.to.field` name (see
	// getScalarTypesAndObjects) so that distinct objects can never overwrite one
	// another; they are emitted in phase 2.
	collected := make([]collectionObjects, 0, len(indices))

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
		collected = append(collected, collectionObjects{name: indexName, fields: fields, objects: objects})

		ndcSchema.Collections = append(ndcSchema.Collections, schema.CollectionInfo{
			Name:                  indexName,
			Arguments:             internal.CollectionArgumentsMap,
			Type:                  indexName,
			UniquenessConstraints: schema.CollectionInfoUniquenessConstraints{},
			ForeignKeys:           schema.CollectionInfoForeignKeys{},
		})
	}

	nativeQueries := configuration.Queries
	parseNativeQueryToSchema(&ndcSchema, state, nativeQueries, &collected)

	// Phase 2: emit the object types. Every nested object type is emitted under
	// its fully-qualified `index.path.to.field` name (assigned in phase 1), and
	// object field references already point at those names, so no rewrite is
	// needed here.
	//
	// Nested object types are ALWAYS fully-qualified — we never collapse them to
	// the bare field name, even when a name is unique. Collapsing would make the
	// generated NDC/GraphQL surface unstable: adding a new index that happens to
	// reuse a nested field name would retroactively rename an existing object
	// type and break consumers. Always-qualified names guarantee that adding an
	// index never renames an existing type, that no fields are ever dropped, and
	// that the output is deterministic (every name derives from mapping paths,
	// independent of Go map iteration order).
	for _, c := range collected {
		prepareNdcSchema(&ndcSchema, c.name, c.fields, c.objects)
	}

	return &ndcSchema
}

// parseNativeQueryToSchema parses the given native queries and adds them to the schema response.
// It also handles return types of kind "defination" and updates the state accordingly.
func parseNativeQueryToSchema(schemaResponse *schema.SchemaResponse, state *types.State, nativeQueries map[string]types.NativeQuery, collected *[]collectionObjects) {
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
			// Defer object-type emission to phase 2 (see ParseConfigurationToSchema)
			// so native-query object types are emitted with the same fully-qualified
			// names as index-derived ones.
			fields, objects := getScalarTypesAndObjects(properties, state, indexName, "")
			*collected = append(*collected, collectionObjects{name: indexName, fields: fields, objects: objects})
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

			flds, objs := getScalarTypesAndObjects(nestedObject, state, indexName, fieldWithParent)

			// Each object type is identified by a fully-qualified, globally-unique
			// name (`index.path.to.field`) and is always emitted under that name.
			// This keeps type names stable: an object in one index can never
			// overwrite — nor be renamed by the later addition of — an
			// identically-named object elsewhere.
			qualifiedName := indexName + "." + fieldWithParent

			fields = append(fields, map[string]interface{}{
				"name": fieldName,
				"type": qualifiedName,
				"obj":  true,
			})

			objects = append(objects, map[string]interface{}{
				"name":   qualifiedName,
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
