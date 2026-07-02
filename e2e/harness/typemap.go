//go:build e2e

package harness

// This file encodes the fixed Elasticsearch -> NDC type table used by the L3
// schema-conformance assertion. It is derived directly from the connector's own
// source of truth:
//
//   - internal/static_types.go : ScalarTypeMap (leaf ES type -> NDC scalar +
//     representation) and the JSON-scalar "object types as scalars" block
//     (geo_point, geo_shape, ip_range, etc.).
//   - internal/static_types.go : TypePriorityMap + internal.SortTypesByPriority,
//     which govern how multi-field (sub-field) compound scalar type names are
//     built (e.g. a `text` field with a `keyword` sub-field becomes the compound
//     NDC scalar type "text.keyword").
//   - connector/schema.go : object/nested handling. `object` mappings become a
//     named NDC object type; `nested` mappings become an ARRAY of that object
//     type. `flattened` is not expanded (treated as JSON).
//
// We intentionally assert against representations (the observable NDC contract)
// rather than trying to byte-for-byte reconstruct compound scalar type names,
// because representations are stable and unambiguous. The compound name itself
// is validated structurally (prefix == base ES type) in schema_assert.go.

// NDC scalar representation type-tags as emitted by ndc-sdk-go
// (schema.TypeRepresentation ... .Type). These are the strings that appear under
// scalar_types.<name>.representation.type in GET :8080/schema.
const (
	reprString     = "string"
	reprInt8       = "int8"
	reprInt16      = "int16"
	reprInt32      = "int32"
	reprInt64      = "int64"
	reprFloat32    = "float32"
	reprFloat64    = "float64"
	reprBigInteger = "biginteger"
	reprBoolean    = "boolean"
	reprJSON       = "json"
)

// esScalarRepr maps a leaf Elasticsearch field type to the NDC scalar
// representation the connector is expected to assign it. Mirrors
// internal.ScalarTypeMap.Representation exactly.
var esScalarRepr = map[string]string{
	// integers
	"byte":          reprInt8,
	"short":         reprInt16,
	"integer":       reprInt32,
	"token_count":   reprInt32, // NewTypeRepresentationInteger() -> int32-family "integer"
	"long":          reprInt64,
	"unsigned_long": reprBigInteger,

	// floats
	"half_float":   reprFloat32,
	"float":        reprFloat32,
	"double":       reprFloat64,
	"scaled_float": reprFloat64,

	// booleans
	"boolean": reprBoolean,

	// strings & string-like
	"keyword":            reprString,
	"text":               reprString,
	"constant_keyword":   reprString,
	"wildcard":           reprString,
	"match_only_text":    reprString,
	"search_as_you_type": reprString,
	"completion":         reprString,
	"binary":             reprString,
	"version":            reprString,
	"ip":                 reprString,

	// dates are represented as strings by the connector
	"date":       reprString,
	"date_nanos": reprString,

	// object-ish / vector / range / geo types surfaced as JSON scalars
	// (the "object types as JSON scalars" block in static_types.go)
	"json":          reprJSON,
	"geo_point":     reprJSON,
	"geo_shape":     reprJSON,
	"point":         reprJSON,
	"shape":         reprJSON,
	"sparse_vector": reprJSON,
	"dense_vector":  reprJSON,
	"rank_feature":  reprJSON,
	"rank_features": reprJSON,
	"percolator":    reprJSON,
	"join":          reprJSON,
	"integer_range": reprJSON,
	"float_range":   reprJSON,
	"long_range":    reprJSON,
	"double_range":  reprJSON,
	"date_range":    reprJSON,
	"ip_range":      reprJSON,
	"alias":         reprJSON,
}

// typePriority mirrors internal.TypePriorityMap. Used to order sub-field types
// when constructing/validating a compound scalar type name.
var typePriority = map[string]int{
	"binary":  1,
	"boolean": 2,

	"date":          11,
	"float":         10,
	"double":        11,
	"integer":       10,
	"long":          11,
	"date_nanos":    11,
	"scaled_float":  10,
	"unsigned_long": 11,
	"short":         10,
	"byte":          10,
	"half_float":    10,

	"ip":               20,
	"version":          20,
	"match_only_text":  20,
	"wildcard":         20,
	"constant_keyword": 20,
	"keyword":          20,
	"text":             20,
	"_id":              20,
}

// isContainerType reports whether an ES field type is an object container that
// the connector expands into a named NDC object type rather than a scalar.
// `flattened` is deliberately NOT here: the connector treats it as opaque.
func isContainerType(esType string) bool {
	return esType == "object" || esType == "nested"
}

// expectedReprFor returns the expected NDC scalar representation for a leaf ES
// type, and whether the type is known to the table. Unknown/opaque types
// (e.g. flattened) return ("", false) and are asserted more loosely.
func expectedReprFor(esType string) (string, bool) {
	r, ok := esScalarRepr[esType]
	return r, ok
}
