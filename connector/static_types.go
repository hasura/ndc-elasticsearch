package connector

import "github.com/hasura/ndc-sdk-go/schema"

var scalarTypeMap = map[string]schema.ScalarType{
	"integer": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("integer"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"long": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
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
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("keyword"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"date": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("date"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"half_float": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("half_float"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"byte": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("byte"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"boolean": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("boolean"),
		Representation:      schema.NewTypeRepresentationBoolean().Encode(),
	},
	"binary": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: map[string]schema.ComparisonOperatorDefinition{},
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"constant_keyword": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("constant_keyword"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"wildcard": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("wildcard"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"short": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("short"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"unsigned_long": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("unsigned_long"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"float": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("float"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"double": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("double"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"scaled_float": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("scaled_float"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"match_only_text": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("match_only_text"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"date_nanos": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("date_nanos"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"ip": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("ip"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"version": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
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
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: getComparisonOperatorDefinition("token_count"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
}

var objectTypeMap = map[string]schema.ObjectType{
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
	"aggregate_metric_double": {
		Fields: schema.ObjectTypeFields{},
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
