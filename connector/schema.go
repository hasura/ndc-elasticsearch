package connector

import (
	"context"
	"sort"
	"strings"

	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// collectionObjects holds the intermediate field/object lists for a single
// collection before name resolution. We collect all collections first, resolve
// object-type names globally, then emit — this prevents the name-collision bug
// where identically-named nested objects across indices (or within one index)
// overwrote each other in the global ObjectTypes map and silently dropped fields.
type collectionObjects struct {
	name    string                   // index / native-query name
	fields  []map[string]interface{} // top-level fields of the collection
	objects []map[string]interface{} // all nested object types (flattened)
}

// GetSchema returns the schema by parsing the configuration.
func (c *Connector) GetSchema(ctx context.Context, configuration *types.Configuration, state *types.State) (schema.SchemaResponseMarshaler, error) {
	return state.Schema, nil
}

// ParseConfigurationToSchema parses the given configuration to generate the schema response.
func ParseConfigurationToSchema(configuration *types.Configuration, state *types.State) *schema.SchemaResponse {
	ndcSchema := schema.SchemaResponse{
		ScalarTypes: make(schema.SchemaResponseScalarTypes),
		ObjectTypes: make(schema.SchemaResponseObjectTypes),
		Collections: []schema.CollectionInfo{},
		Functions:   []schema.FunctionInfo{},
		Procedures:  []schema.ProcedureInfo{},
	}

	indices := configuration.Indices

	// Phase 1: walk every index and collect fields/objects without emitting
	// object types yet. reservedNames tracks names that a nested object type
	// must not reuse as its bare name (index names and static type names).
	collected := make([]collectionObjects, 0, len(indices))
	reservedNames := make(map[string]bool)

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
		reservedNames[indexName] = true

		ndcSchema.Collections = append(ndcSchema.Collections, schema.CollectionInfo{
			Name:                  indexName,
			Arguments:             internal.CollectionArgumentsMap,
			Type:                  indexName,
			UniquenessConstraints: schema.CollectionInfoUniquenessConstraints{},
			ForeignKeys:           schema.CollectionInfoForeignKeys{},
		})
	}

	nativeQueries := configuration.Queries
	parseNativeQueryToSchema(&ndcSchema, state, nativeQueries, &collected, reservedNames)

	// Phase 2: resolve object-type names globally. Bare name is kept when
	// unambiguous; fully-qualified name (index.path.to.field) is used only
	// when the bare name maps to two or more distinct structures.
	resolution := resolveObjectTypeNames(collected, reservedNames)

	// Phase 3: rewrite every object-type reference to its resolved name, then emit.
	for _, c := range collected {
		rewriteObjectFieldTypes(c.fields, resolution)
		for _, obj := range c.objects {
			rewriteObjectFieldTypes(obj["fields"].([]map[string]interface{}), resolution)
			if qualified, ok := obj["name"].(string); ok {
				if final, ok := resolution[qualified]; ok {
					obj["name"] = final
				}
			}
		}
		prepareNdcSchema(&ndcSchema, c.name, c.fields, c.objects)
	}

	return &ndcSchema
}

// resolveObjectTypeNames builds a map from each object's fully-qualified name
// to the name it should be emitted under.
//
// An object keeps its bare field name when that bare name maps to exactly one
// distinct structure and does not collide with a reserved name. Otherwise every
// object sharing that bare name is emitted under its fully-qualified name.
func resolveObjectTypeNames(collected []collectionObjects, reservedNames map[string]bool) map[string]string {
	// Add static type names so a nested object never shadows them.
	for name := range internal.ScalarTypeMap {
		reservedNames[name] = true
	}
	for name := range internal.ObjectTypeMap {
		reservedNames[name] = true
	}
	for name := range internal.RequiredScalarTypes {
		reservedNames[name] = true
	}
	for name := range internal.RequiredObjectTypes {
		reservedNames[name] = true
	}
	reservedNames["_id"] = true

	// Collect the set of distinct structural signatures seen per bare name.
	signaturesByBareName := make(map[string]map[string]bool)
	for _, c := range collected {
		for _, obj := range c.objects {
			bare := obj["bareName"].(string)
			sig := obj["signature"].(string)
			if signaturesByBareName[bare] == nil {
				signaturesByBareName[bare] = make(map[string]bool)
			}
			signaturesByBareName[bare][sig] = true
		}
	}

	resolution := make(map[string]string)
	for _, c := range collected {
		for _, obj := range c.objects {
			qualified := obj["name"].(string)
			bare := obj["bareName"].(string)
			if len(signaturesByBareName[bare]) == 1 && !reservedNames[bare] {
				resolution[qualified] = bare
			} else {
				resolution[qualified] = qualified
			}
		}
	}
	return resolution
}

// rewriteObjectFieldTypes replaces the type of every object field with its
// resolved name. Scalar field types are not in the resolution map and are
// left untouched.
func rewriteObjectFieldTypes(fields []map[string]interface{}, resolution map[string]string) {
	for _, field := range fields {
		if _, isObject := field["obj"]; !isObject {
			continue
		}
		if qualified, ok := field["type"].(string); ok {
			if final, ok := resolution[qualified]; ok {
				field["type"] = final
			}
		}
	}
}

// parseNativeQueryToSchema parses the given native queries and adds them to the schema response.
// It also handles return types of kind "defination" and updates the state accordingly.
func parseNativeQueryToSchema(schemaResponse *schema.SchemaResponse, state *types.State, nativeQueries map[string]types.NativeQuery, collected *[]collectionObjects, reservedNames map[string]bool) {
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
			// Defer emission to phase 3 so native-query objects join global name resolution.
			fields, objects := getScalarTypesAndObjects(properties, state, indexName, "")
			*collected = append(*collected, collectionObjects{name: indexName, fields: fields, objects: objects})
			reservedNames[indexName] = true
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
// Each nested object is tagged with a fully-qualified name (index.path.to.field),
// a bare name (the field name alone), and a structural signature. Name resolution
// happens later in resolveObjectTypeNames.
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

			// Qualify the name globally; resolution may later collapse it back to
			// the bare field name when there is no ambiguity.
			qualifiedName := indexName + "." + fieldWithParent
			sig := objectSignature(flds)

			fields = append(fields, map[string]interface{}{
				"name":      fieldName,
				"type":      qualifiedName,
				"obj":       true,
				"signature": sig,
			})

			objects = append(objects, map[string]interface{}{
				"name":      qualifiedName,
				"bareName":  fieldName,
				"signature": sig,
				"fields":    flds,
			})
			objects = append(objects, objs...)
		}
	}
	return fields, objects
}

// objectSignature returns an order-independent, recursive fingerprint of an
// object's fields. Two objects produce the same signature only when they have
// identical field names, scalar types, and nested structures. It is used to
// decide whether two objects sharing a bare field name are in fact the same
// type (safe to collapse to a bare name) or different types (must be kept
// under their fully-qualified names).
func objectSignature(fields []map[string]interface{}) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		name, _ := field["name"].(string)
		if _, isObject := field["obj"]; isObject {
			// Embed the child's own signature so the fingerprint is recursive
			// and independent of the (qualified) child type name.
			childSig, _ := field["signature"].(string)
			parts = append(parts, name+"=@{"+childSig+"}")
		} else {
			fieldType, _ := field["type"].(string)
			parts = append(parts, name+"="+fieldType)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
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

	// Add the required scalar type to the schema
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
