package connector

import "github.com/hasura/ndc-sdk-go/schema"

var scalarTypeMap = map[string]schema.ScalarType{
	"integer": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("integer"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"long": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("long"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"text": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("text"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"keyword": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("keyword"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"date": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("date"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"half_float": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("half_float"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"byte": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("byte"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"boolean": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("boolean"),
		Representation:      schema.NewTypeRepresentationBoolean().Encode(),
	},
	"binary": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: map[string]schema.ComparisonOperatorDefinition{},
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"constant_keyword": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("constant_keyword"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"wildcard": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("wildcard"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"short": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("short"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"unsigned_long": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("unsigned_long"),
		Representation:      schema.NewTypeRepresentationInteger().Encode(),
	},
	"float": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("float"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"double": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("double"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"scaled_float": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("scaled_float"),
		Representation:      schema.NewTypeRepresentationNumber().Encode(),
	},
	"match_only_text": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("match_only_text"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"date_nanos": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("date_nanos"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"ip": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("ip"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"version": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("version"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"completion": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("completion"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"search_as_you_type": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("search_as_you_type"),
		Representation:      schema.NewTypeRepresentationString().Encode(),
	},
	"token_count": {
		AggregateFunctions:  schema.ScalarTypeAggregateFunctions{},
		ComparisonOperators: comparisonOperatorDefinition("token_count"),
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
			"values": {
				Type: schema.NewArrayType(schema.NewNamedType("integer")).Encode(),
			},
			"counts": {
				Type: schema.NewArrayType(schema.NewNamedType("integer")).Encode(),
			},
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
	"flattened": {
		Fields: schema.ObjectTypeFields{},
	},
}

var comparisonOperatorDefinition = func(dataType string) map[string]schema.ComparisonOperatorDefinition {
	return map[string]schema.ComparisonOperatorDefinition{
		"match":               schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"match_phrase":        schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"match_phrase_prefix": schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"match_bool":          schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"term":                schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"exists":              schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"prefix":              schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"wildcard":            schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"regexp":              schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"fuzzy":               schema.NewComparisonOperatorCustom(schema.NewNamedType(dataType)).Encode(),
		"terms":               schema.NewComparisonOperatorCustom(schema.NewArrayType(schema.NewNamedType(dataType))).Encode(),
	}
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
}
