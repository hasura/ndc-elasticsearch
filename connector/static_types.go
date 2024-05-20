package connector

import "github.com/hasura/ndc-sdk-go/schema"

var scalarTypeMap = map[string]schema.ScalarType{
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

var objectTypeMap = map[string]schema.ObjectType{
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
			"values": schema.ObjectField{Type: schema.NewNamedType("float").Encode()},
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
}

// getComparisonOperatorDefinition generates and returns a map of comparison operators based on the provided data type.
func getComparisonOperatorDefinition(dataType string) map[string]schema.ComparisonOperatorDefinition {
	var comparisonOperators = map[string]schema.ComparisonOperatorDefinition{
		"match":        schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"match_phrase": schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"term":         schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"terms":        schema.NewComparisonOperatorCustom(schema.NewArrayType(schema.NewNamedType(dataType))).Encode(),
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
	if dataType == "_id" {
		comparisonOperators["term"] = schema.NewComparisonOperatorEqual().Encode()
	}
	return comparisonOperators
}

func getAggregationFunctions(functions []string, typeName string) schema.ScalarTypeAggregateFunctions {
	aggregationFunctions := make(schema.ScalarTypeAggregateFunctions)
	for _, function := range functions {
		if function == "cardinality" || function == "value_count" {
			typeName = "integer"
		}
		if function == "stats" || function == "string_stats" {
			typeName = function
		}
		aggregationFunctions[function] = schema.AggregateFunctionDefinition{
			ResultType: schema.NewNamedType(typeName).Encode(),
		}
	}
	return aggregationFunctions
}

var unSupportedAggregateTypes = []string{
	"text",
	"search_as_you_type",
	"completion",
	"match_only_text",
	"binary",
}

var unsupportedSortDataTypes = []string{
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
