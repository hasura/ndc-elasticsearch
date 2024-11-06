package connector

import (
	"fmt"
	"strings"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-sdk-go/schema"
)

/**
 * This function checks if a field is a scalar field
 * Scalar field here refers to a field that does not have any nested fields/properties
 */
func fieldIsScalar(fieldMap map[string]interface{}) (fieldType string, isFieldScalar bool) {
	fieldType, ok := fieldMap["type"].(string)
	return fieldType, ok && fieldType != "nested" && fieldType != "object" && fieldType != "flattened"
}

func fieldTypeIsAggregateMetricDouble(fieldType string) bool {
	return fieldType == "aggregate_metric_double"
}

func handleFieldTypeAggregateMetricDouble(fieldMap map[string]interface{}) {
	const fieldType = "aggregate_metric_double"

	metrics, ok := fieldMap["metrics"].([]interface{})
	metricFields := schema.ObjectTypeFields{}
	if ok {
		for _, metric := range metrics {
			if metricValue, ok := metric.(string); ok {
				metricFields[metricValue] = schema.ObjectField{Type: schema.NewNamedType("double").Encode()}
			}
		}
		internal.ObjectTypeMap[fieldType] = schema.ObjectType{
			Fields: metricFields,
		}
	}
}

// `GetFieldType` returns the type of the field. It also handles fields with subtypes
// It also handles the check for whether a given fields' type supports comparison and aggregation operations
//
// # For scalar fields that have no subtypes, their types are returned after checks for comparison and aggregation operations
//
// For fields that have subtypes, the types are sorted by priority and then the comparison and aggregation operations are checked for each subtype
// A compound scalar type is generated for these types. The compound scalar type has the format `actualFieldType.subtype1.subtype2...`
// This compound scalar type supports a superset of comparison and aggregation operations of all its subtypes and the actualType
// This compund scalar type is added to the scalarTypeMap before being returned
func GetFieldType(fieldMap map[string]interface{}, state *types.State, indexName string, fieldName string) string {
	fieldTypes := internal.ExtractTypes(fieldMap)
	actualFieldType := fieldTypes[0] // actualFieldType is the type type of the field that the db has. It is the main type, not the subtype

	if len(fieldTypes) > 1 {
		// subtypes present
		// we need to sort fields by priority
		// because the fields that can represent the most format of data should be added at the end,
		// so that their comparison operators are not overridden by the fields that can represent less formats and same for aggregate functions
		internal.SortTypesByPriority(fieldTypes)
	}

	allSupportedComparisonOperations := make(map[string]schema.ComparisonOperatorDefinition)
	allSupportedAggregationOperations := make(schema.ScalarTypeAggregateFunctions)
	unstructuredTextSupported := false
	termLevelQueriesSupported := false

	for _, currentType := range fieldTypes {
		supportedComparisionOperations, supportedAggregationOperations, curUnstructuredTextSupported, curTermLevelQueriesSupported := processFieldType(fieldMap, currentType)

		if curUnstructuredTextSupported {
			unstructuredTextSupported = true
		}

		if curTermLevelQueriesSupported {
			termLevelQueriesSupported = true
		}

		allSupportedComparisonOperations = appendComparisonOperations(allSupportedComparisonOperations, supportedComparisionOperations)
		allSupportedAggregationOperations = appendAggregationOperations(allSupportedAggregationOperations, supportedAggregationOperations)
	}

	if len(allSupportedComparisonOperations) > 0 {
		state.SupportedSortFields[indexName].(map[string]string)[fieldName] = fieldName
	}

	if len(allSupportedAggregationOperations) > 0 {
		state.SupportedAggregateFields[indexName].(map[string]string)[fieldName] = fieldName
	}

	if unstructuredTextSupported {
		state.SupportedFilterFields[indexName].(map[string]interface{})["unstructured_text"].(map[string]string)[fieldName] = fieldName
	}

	if termLevelQueriesSupported {
		state.SupportedFilterFields[indexName].(map[string]interface{})["term_level_queries"].(map[string]string)[fieldName] = fieldName
	}

	if len(fieldTypes) == 1 {
		// no subfields present, return the actualFieldType
		return actualFieldType
	}

	// create a compound scalar type

	// this slice will contain only the subtypes
	subtypes := []string{}
	for _, fieldType := range fieldTypes {
		if fieldType == actualFieldType {
			continue
		}
		subtypes = append(subtypes, fieldType)
	}
	// the new compound scalar type must have the format `actualFieldType.subtype1.subtype2...`
	scalarType := fmt.Sprintf("%s.%s", actualFieldType, strings.Join(subtypes, "."))

	// since a new compound scalar type has been created, it must be added to the scalarTypeMap
	appendCompoundTypeToStaticTypes(scalarType, allSupportedComparisonOperations, allSupportedAggregationOperations, actualFieldType)
	return scalarType
}

func appendCompoundTypeToStaticTypes(typeName string, sortOperations map[string]schema.ComparisonOperatorDefinition, aggegateOperations schema.ScalarTypeAggregateFunctions, actualFieldType string) {
	internal.ScalarTypeMap[typeName] = schema.ScalarType{
		AggregateFunctions:  aggegateOperations,
		ComparisonOperators: sortOperations,
		Representation:      internal.ScalarTypeMap[actualFieldType].Representation,
	}
}


// This function takes a fieldType and checks whether it
// 1. supports comparison operations
// 2. supports aggregation operations
// 3. supports unstructured text search
// 4. supports term level queries
//
// It also handles the case where the fieldType is "aggregate_metric_double"
func processFieldType(fieldMap map[string]interface{}, fieldType string) (supportedComparisionOperations map[string]schema.ComparisonOperatorDefinition, supportedAggregationOperations schema.ScalarTypeAggregateFunctions, unstructuredTextSupported bool, termLevelQueriesSupported bool) {
	if fieldTypeIsAggregateMetricDouble(fieldType) {
		handleFieldTypeAggregateMetricDouble(fieldMap)
	}

	fieldDataEnalbed := false // TODO: for now, we won't support field data inside nested types (subtypes)
	if fieldData, ok := fieldMap["fielddata"].(bool); ok {
		fieldDataEnalbed = fieldData
	}

	if internal.IsSortSupported(fieldType, fieldDataEnalbed) {
		supportedComparisionOperations = internal.ScalarTypeMap[fieldType].ComparisonOperators
	}
	if internal.IsAggregateSupported(fieldType, fieldDataEnalbed) {
		supportedAggregationOperations = internal.ScalarTypeMap[fieldType].AggregateFunctions
	}
	if fieldType == "wildcard" {
		unstructuredTextSupported = true
	}
	if fieldType == "keyword" {
		termLevelQueriesSupported = true
	}

	return supportedComparisionOperations, supportedAggregationOperations, unstructuredTextSupported, termLevelQueriesSupported
}

func appendComparisonOperations(supersetSortOps map[string]schema.ComparisonOperatorDefinition, localSortOps map[string]schema.ComparisonOperatorDefinition) map[string]schema.ComparisonOperatorDefinition {
	for localSortOpName, localSortOp := range localSortOps {
		supersetSortOps[localSortOpName] = localSortOp
	}
	return supersetSortOps
}

func appendAggregationOperations(supersetAggOps schema.ScalarTypeAggregateFunctions, localAggOps schema.ScalarTypeAggregateFunctions) schema.ScalarTypeAggregateFunctions {
	for aggFuncName, aggFuncDefinition := range localAggOps {
		supersetAggOps[aggFuncName] = aggFuncDefinition
	}
	return supersetAggOps
}
