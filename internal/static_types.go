package internal

import (
	"slices"

	"github.com/hasura/ndc-sdk-go/schema"
	"github.com/hasura/ndc-sdk-go/utils"
)

var NumericFields = []string{"integer", "long", "short", "byte", "halft_float", "unsigned_long", "float", "double", "scaled_float"}

var ValidFunctions = []string{"sum", "min", "max", "avg", "value_count", "cardinality", "stats", "string_stats"}

var ScalarTypeMap = map[string]schema.ScalarType{
	"integer": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "integer"),
		ComparisonOperators: getComparisonOperatorDefinition("integer"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"long": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "long"),
		ComparisonOperators: getComparisonOperatorDefinition("long"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"text": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("text"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"_id": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("_id"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"keyword": {
		AggregateFunctions:  getAggregationFunctions([]string{"value_count", "cardinality", "string_stats"}, "keyword"),
		ComparisonOperators: getComparisonOperatorDefinition("keyword"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"date": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "long"),
		ComparisonOperators: getComparisonOperatorDefinition("date"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"half_float": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "half_float"),
		ComparisonOperators: getComparisonOperatorDefinition("half_float"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"byte": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "byte"),
		ComparisonOperators: getComparisonOperatorDefinition("byte"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"boolean": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "integer"),
		ComparisonOperators: getComparisonOperatorDefinition("boolean"),
		Representation:      schema.NewTypeRepresentationBoolean().Encode(),
	},
	"binary": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: map[string]schema.ComparisonOperatorDefinition{},
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"constant_keyword": {
		AggregateFunctions:  getAggregationFunctions([]string{"value_count", "cardinality", "string_stats"}, "constant_keyword"),
		ComparisonOperators: getComparisonOperatorDefinition("constant_keyword"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"wildcard": {
		AggregateFunctions:  getAggregationFunctions([]string{"value_count", "cardinality", "string_stats"}, "integer"),
		ComparisonOperators: getComparisonOperatorDefinition("wildcard"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"short": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "short"),
		ComparisonOperators: getComparisonOperatorDefinition("short"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"unsigned_long": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "unsigned_long"),
		ComparisonOperators: getComparisonOperatorDefinition("unsigned_long"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"float": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "float"),
		ComparisonOperators: getComparisonOperatorDefinition("float"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"double": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "double"),
		ComparisonOperators: getComparisonOperatorDefinition("double"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"scaled_float": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "scaled_float"),
		ComparisonOperators: getComparisonOperatorDefinition("scaled_float"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"match_only_text": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("match_only_text"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"date_nanos": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "long"),
		ComparisonOperators: getComparisonOperatorDefinition("date_nanos"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"ip": {
		AggregateFunctions:  getAggregationFunctions([]string{"value_count", "cardinality"}, "ip"),
		ComparisonOperators: getComparisonOperatorDefinition("ip"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"version": {
		AggregateFunctions:  getAggregationFunctions([]string{"value_count", "cardinality"}, "version"),
		ComparisonOperators: getComparisonOperatorDefinition("version"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"completion": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("completion"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"search_as_you_type": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("search_as_you_type"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"token_count": {
		AggregateFunctions:  getAggregationFunctions([]string{"max", "min", "sum", "avg", "value_count", "cardinality", "stats"}, "integer"),
		ComparisonOperators: getComparisonOperatorDefinition("token_count"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
}

// typePriorityMap is a map of data types and their priority for sorting.
// The 'priority' of a type refers to how encapsulating of other types it is.
// It means can a type represent another type or not.
//
// For example, 'string' is a higher priority than float, because every float can be represented as a string, but not vice versa.
var TypePriorityMap = map[string]int{
	"binary":  1,
	"boolean": 2,

	// numeric representations, second highest priority
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

	// string representations, highest priority
	"ip":               20,
	"version":          20,
	"match_only_text":  20,
	"wildcard":         20,
	"constant_keyword": 20,
	"keyword":          20,
	"text":             20,
	"_id":              20,
}

var RequiredScalarTypes = map[string]schema.ScalarType{
	"double":  ScalarTypeMap["double"],
	"integer": ScalarTypeMap["integer"],
	"float":   ScalarTypeMap["float"],
	"keyword": ScalarTypeMap["keyword"],
	"long":    ScalarTypeMap["long"],
}

var RequiredObjectTypes = map[string]schema.ObjectType{
	"stats": {
		Fields: schema.ObjectTypeFields{
			"count": schema.ObjectField{
				Type: schema.NewNamedType("integer").Encode(),
			},
			"min": schema.ObjectField{
				Type: schema.NewNamedType("double").Encode(),
			},
			"max": schema.ObjectField{
				Type: schema.NewNamedType("double").Encode(),
			},
			"avg": schema.ObjectField{
				Type: schema.NewNamedType("double").Encode(),
			},
			"sum": schema.ObjectField{
				Type: schema.NewNamedType("double").Encode(),
			},
		},
	},
	"string_stats": {
		Fields: schema.ObjectTypeFields{
			"count": schema.ObjectField{
				Type: schema.NewNamedType("integer").Encode(),
			},
			"min_length": schema.ObjectField{
				Type: schema.NewNamedType("integer").Encode(),
			},
			"max_length": schema.ObjectField{
				Type: schema.NewNamedType("integer").Encode(),
			},
			"avg_length": schema.ObjectField{
				Type: schema.NewNamedType("double").Encode(),
			},
			"entropy": schema.ObjectField{
				Type: schema.NewNamedType("double").Encode(),
			},
		},
	},
	"range": {
		Fields: schema.ObjectTypeFields{
			"gt": schema.ObjectField{
				Description: utils.ToPtr("(Optional) Greater than."),
				Type:        schema.NewNamedType("double").Encode(),
			},
			"lt": schema.ObjectField{
				Description: utils.ToPtr("(Optional) Less than."),
				Type:        schema.NewNamedType("double").Encode(),
			},
			"gte": schema.ObjectField{
				Description: utils.ToPtr("(Optional) Greater than or equal."),
				Type:        schema.NewNamedType("double").Encode(),
			},
			"lte": schema.ObjectField{
				Description: utils.ToPtr("(Optional) Less than or equal."),
				Type:        schema.NewNamedType("double").Encode(),
			},
			"boost": schema.ObjectField{
				Description: utils.ToPtr("(Optional, float) Floating point number used to decrease or increase the relevance scores of a query. Defaults to 1.0."),
				Type:        schema.NewNamedType("float").Encode(),
			},
		},
	},
}

var ObjectTypeMap = map[string]schema.ObjectType{
	"sparse_vector": {
		Fields: schema.ObjectTypeFields{},
	},
	"dense_vector": {
		Fields: schema.ObjectTypeFields{},
	},
	"rank_feature": {
		Fields: schema.ObjectTypeFields{},
	},
	"rank_features": {
		Fields: schema.ObjectTypeFields{},
	},
	"percolator": {
		Fields: schema.ObjectTypeFields{},
	},
	"histogram": {
		Fields: schema.ObjectTypeFields{
			"values": schema.ObjectField{Type: schema.NewNamedType("double").Encode()},
			"counts": schema.ObjectField{Type: schema.NewNamedType("integer").Encode()},
		},
	},
	"geo_point": {
		Fields: schema.ObjectTypeFields{},
	},
	"geo_shape": {
		Fields: schema.ObjectTypeFields{},
	},
	"join": {
		Fields: schema.ObjectTypeFields{},
	},
	"integer_range": {
		Fields: schema.ObjectTypeFields{},
	},
	"float_range": {
		Fields: schema.ObjectTypeFields{},
	},
	"long_range": {
		Fields: schema.ObjectTypeFields{},
	},
	"double_range": {
		Fields: schema.ObjectTypeFields{},
	},
	"date_range": {
		Fields: schema.ObjectTypeFields{},
	},
	"ip_range": {
		Fields: schema.ObjectTypeFields{},
	},
	"point": {
		Fields: schema.ObjectTypeFields{},
	},
	"shape": {
		Fields: schema.ObjectTypeFields{},
	},
	"alias": {
		Fields: schema.ObjectTypeFields{},
	},
	"date_range_query": {
		Fields: schema.ObjectTypeFields{
			"gt": schema.ObjectField{
				Description: utils.ToPtr("(Optional) Greater than."),
				Type:        schema.NewNamedType("double").Encode(),
			},
			"lt": schema.ObjectField{
				Description: utils.ToPtr("(Optional) Less than."),
				Type:        schema.NewNamedType("double").Encode(),
			},
			"gte": schema.ObjectField{
				Description: utils.ToPtr("(Optional) Greater than or equal."),
				Type:        schema.NewNamedType("double").Encode(),
			},
			"lte": schema.ObjectField{
				Description: utils.ToPtr("(Optional) Less than or equal."),
				Type:        schema.NewNamedType("double").Encode(),
			},
			"format": schema.ObjectField{
				Description: utils.ToPtr("(Optional, string) Date format used to convert date values in the query."),
				Type:        schema.NewNamedType("keyword").Encode(),
			},
			"boost": schema.ObjectField{
				Description: utils.ToPtr("(Optional, float) Floating point number used to decrease or increase the relevance scores of a query. Defaults to 1.0."),
				Type:        schema.NewNamedType("float").Encode(),
			},
			"time_zone": schema.ObjectField{
				Description: utils.ToPtr("(Optional, string) Coordinated Universal Time (UTC) offset or IANA time zone used to convert date values in the query to UTC."),
				Type:        schema.NewNamedType("keyword").Encode(),
			},
		},
	},
}

var UnsupportedRangeQueryScalars = []string{"binary", "completion", "_id", "wildcard", "match_only_text", "search_as_you_type"}

// getComparisonOperatorDefinition generates and returns a map of comparison operators based on the provided data type.
func getComparisonOperatorDefinition(dataType string) map[string]schema.ComparisonOperatorDefinition {
	var comparisonOperators = map[string]schema.ComparisonOperatorDefinition{
		"match":        schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"match_phrase": schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"term":         schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"terms":        schema.NewComparisonOperatorCustom(schema.NewArrayType(schema.NewNamedType(dataType))).Encode(),
	}

	if !slices.Contains(UnsupportedRangeQueryScalars, dataType) {
		comparisonOperators["range"] = schema.NewComparisonOperatorCustom(schema.NewNamedType("range")).Encode()
	}

	if dataType == "date" {
		RequiredObjectTypes["date_range_query"] = ObjectTypeMap["date_range_query"]
		comparisonOperators["range"] = schema.NewComparisonOperatorCustom(schema.NewNamedType("date_range_query")).Encode()
	}

	if dataType == "text" {
		comparisonOperators["match_phrase_prefix"] = schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode()
	}

	if dataType == "text" || dataType == "keyword" || dataType == "wildcard" {
		comparisonOperators["wildcard"] = schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode()
		comparisonOperators["regexp"] = schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode()
		comparisonOperators["prefix"] = schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode()
		comparisonOperators["match_bool_prefix"] = schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode()
	}

	comparisonOperators["term"] = schema.NewComparisonOperatorEqual().Encode()

	return comparisonOperators
}

// getAggregationFunctions generates and returns a map of aggregation functions based on the provided list of functions and data type.
func getAggregationFunctions(functions []string, typeName string) schema.ScalarTypeAggregateFunctions {
	aggregationFunctions := make(schema.ScalarTypeAggregateFunctions)

	for _, function := range functions {
		if function == "cardinality" || function == "value_count" {
			typeName = "integer"
		} else if function == "stats" || function == "string_stats" {
			typeName = function
		}

		// Generate the function definition and add it to the map
		aggregationFunctions[function] = schema.AggregateFunctionDefinition{
			ResultType: schema.NewNamedType(typeName).Encode(),
		}
	}

	return aggregationFunctions
}

func SortTypesByPriority(types []string) {
	for i := 0; i < len(types); i++ {
		for j := i + 1; j < len(types); j++ {
			if TypePriorityMap[types[i]] > TypePriorityMap[types[j]] {
				types[i], types[j] = types[j], types[i]
			} else if TypePriorityMap[types[i]] == TypePriorityMap[types[j]] {
				// priority is same, sort alphabetically
				if types[i] > types[j] {
					types[i], types[j] = types[j], types[i]
				}
			}
		}
	}
}

// unSupportedAggregateTypes are lists of data types that do not support aggregation in elasticsearch.
var UnSupportedAggregateTypes = []string{
	"text",
	"search_as_you_type",
	"completion",
	"match_only_text",
	"binary",
}

// unSupportedSortDataTypes are lists of data types that do not support sorting in elasticsearch.
var UnSupportedSortDataTypes = []string{
	"text",
	"search_as_you_type",
	"binary",
	"match_only_text",
	"completion",
	"histogram",
	"point",
	"shape",
	"geo_shape",
	"geo_point",
	"rank_feature",
	"rank_features",
	"sparse_vector",
	"dense_vector",
	"percolator",
	"alias",
	"join",
	"range",
	"_id",
}

// TermLevelQueries queries in elasticsearch for keyword family of types
// more reading: https://www.elastic.co/guide/en/elasticsearch/reference/current/term-level-queries.html
var TermLevelQueries = map[string]bool{
	"exists":   true,
	"fuzzy":    true,
	"ids":      true,
	"prefix":   true,
	"range":    true,
	"regexp":   true,
	"term":     true,
	"terms":    true,
	"term_set": true,
	"wildcard": true,
	"__sort":   true, // __sort is a custom operator that represents sorting operation (this is *NOT* an elasticsearch operator)
}

var TermLevelAggregations = map[string]bool{
	"value_count":  true,
	"cardinality":  true,
	"string_stats": true,
}

// range operations are optimzed for numeric types
var NumericalQueries = map[string]bool{
	"range":  true,
	"__sort": true, // __sort is a custom operator that represents sorting operation (this is *NOT* an elasticsearch operator)
}

var NumericalAggregations = map[string]bool{
	"max":         true,
	"min":         true,
	"sum":         true,
	"avg":         true,
	"value_count": true,
	"cardinality": true,
	"stats":       true,
}

// FullTextQueries queries in elasticsearch for text family of types
// more reading: https://www.elastic.co/guide/en/elasticsearch/reference/current/full-text-queries.html
var FullTextQueries = map[string]bool{
	"intervals":           true,
	"match":               true,
	"match_bool_prefix":   true,
	"match_phrase":        true,
	"match_phrase_prefix": true,
	"multi_match":         true,
	"combined_fields":     true,
	"query_string":        true,
	"simple_query_string": true,
}

var FullTextAggregations = map[string]bool{}

// Used for unstructured text, like the body of an email
// more reading: https://www.elastic.co/guide/en/elasticsearch/reference/current/text.html
var TextFamilyOfTypes = map[string]bool{
	"text":            true,
	"match_only_text": true,
	"ip":              true,
}

// Used for structured content like email addresses, hostnames, status codes, zip codes or tags.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/keyword.html
var KeywordFamilyOfTypes = map[string]bool{
	"keyword":          true,
	"constant_keyword": true,
	"wildcard":         true,
	"date":             true,
	"date_nanos":       true,
	"ip":               true,
	"version":          true,
}

// https://www.elastic.co/guide/en/elasticsearch/reference/current/number.html
var NumericFamilyOfTypes = map[string]bool{
	"integer":       true,
	"long":          true,
	"short":         true,
	"byte":          true,
	"double":        true,
	"float":         true,
	"half_float":    true,
	"unsigned_long": true,
	"scaled_float":  true,
	"date":          true,
	"date_nanos":    true,
}
