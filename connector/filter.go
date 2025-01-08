package connector

import (
	"fmt"
	"strings"

	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// prepareFilterQuery prepares a filter query based on the given expression.
func prepareFilterQuery(expression schema.Expression, state *types.State, collection string) (map[string]interface{}, error) {
	filter := make(map[string]interface{})
	switch expr := expression.Interface().(type) {
	case *schema.ExpressionUnaryComparisonOperator:
		return handleExpressionUnaryComparisonOperator(expr, state, collection)
	case *schema.ExpressionBinaryComparisonOperator:
		return handleExpressionBinaryComparisonOperator(expr, state, collection)
	case *schema.ExpressionAnd:
		queries := make([]map[string]interface{}, 0)
		for _, expr := range expr.Expressions {
			res, err := prepareFilterQuery(expr, state, collection)
			if err != nil {
				return nil, err
			}
			queries = append(queries, res)
		}
		filter["bool"] = map[string]interface{}{
			"must": queries,
		}
		return filter, nil
	case *schema.ExpressionOr:
		queries := make([]map[string]interface{}, 0)
		for _, expr := range expr.Expressions {
			res, err := prepareFilterQuery(expr, state, collection)
			if err != nil {
				return nil, err
			}
			queries = append(queries, res)
		}
		filter["bool"] = map[string]interface{}{
			"should": queries,
		}
		return filter, nil
	case *schema.ExpressionNot:
		res, err := prepareFilterQuery(expr.Expression, state, collection)
		if err != nil {
			return nil, err
		}

		filter["bool"] = map[string]interface{}{
			"must_not": res,
		}
		return filter, nil
	default:
		return nil, schema.UnprocessableContentError("invalid predicate type", map[string]any{
			"expression": expression,
		})
	}
}

// handleExpressionUnaryComparisonOperator processes the unary comparison operator expression.
func handleExpressionUnaryComparisonOperator(expr *schema.ExpressionUnaryComparisonOperator, state *types.State, collection string) (map[string]interface{}, error) {
	if expr.Operator == "is_null" {
		fieldName, _ := joinFieldPath(state, expr.Column.FieldPath, expr.Column.Name, collection)
		value := map[string]interface{}{
			"field": fieldName,
		}
		filter := map[string]interface{}{"bool": map[string]interface{}{"must_not": map[string]interface{}{"exists": value}}}
		if nestedFields, ok := state.NestedFields[collection]; ok {
			if _, ok := nestedFields.(map[string]string)[expr.Column.Name]; ok {
				filter = prepareNestedQuery(state, "bool.must_not.exists", value, fieldName, len(expr.Column.FieldPath), collection)
			}
		}
		return filter, nil
	}
	return nil, schema.UnprocessableContentError("invalid unary comparison operator", map[string]any{
		"operator": expr.Operator,
	})
}

// handleExpressionBinaryComparisonOperator processes the binary comparison operator expression.
func handleExpressionBinaryComparisonOperator(
	expr *schema.ExpressionBinaryComparisonOperator,
	state *types.State,
	collection string,
) (map[string]interface{}, error) {
	fieldPath, nestedPath := joinFieldPath(state, expr.Column.FieldPath, expr.Column.Name, collection)
	fieldType, fieldSubTypes, _, err := state.Configuration.GetFieldProperties(collection, fieldPath)
	if err != nil {
		return nil, schema.UnprocessableContentError("unable to get field types", map[string]any{
			"fieldPath": fieldPath,
			"index":     collection,
		})
	}

	bestFieldOrSubFieldFound := false
	var bestFieldOrSubField string

	// we need to check what type or subtype is best optimized for the operator, and use that type or subtype of the field
	if internal.NumericalQueries[expr.Operator] {
		// this is a numerical query, optimized for numeric types
		bestFieldOrSubField, bestFieldOrSubFieldFound = getCorrectFieldForOperator(fieldPath, fieldType, fieldSubTypes, internal.NumericFamilyOfTypes)
	} else if internal.TermLevelQueries[expr.Operator] && !bestFieldOrSubFieldFound {
		// this a term level query, optimized for keyword types
		bestFieldOrSubField, bestFieldOrSubFieldFound = getCorrectFieldForOperator(fieldPath, fieldType, fieldSubTypes, internal.KeywordFamilyOfTypes)
	} else if internal.FullTextQueries[expr.Operator] && !bestFieldOrSubFieldFound {
		// this is a full text query, optimized for text types
		bestFieldOrSubField, bestFieldOrSubFieldFound = getCorrectFieldForOperator(fieldPath, fieldType, fieldSubTypes, internal.TextFamilyOfTypes)
	} else {
		return nil, schema.UnprocessableContentError("invalid binary comaparison operator", map[string]any{
			"expression": expr.Operator,
		})
	}

	value, err := evalComparisonValue(expr.Value, bestFieldOrSubField, expr.Operator)
	if err != nil {
		return nil, err
	}

	filter := map[string]interface{}{
		expr.Operator: value,
	}

	if nestedPath != "" {
		filter = prepareNestedQuery(state, expr.Operator, value, fieldPath, len(expr.Column.FieldPath), collection)
	}

	return filter, nil
}

// joinFieldPath joins the fieldPath and columnName to form a fully qualified field path.
// It also checks if the field is nested and returns the nested path.
func joinFieldPath(state *types.State, fieldPath []string, columnName string, collection string) (string, string) {
	nestedPath := ""

	if nestedFields, ok := state.NestedFields[collection]; ok {
		if _, ok := nestedFields.(map[string]string)[columnName]; ok {
			nestedPath = columnName
		}
	}

	joinedPath := columnName

	for _, field := range fieldPath {
		joinedPath = joinedPath + "." + field

		// Check if the joined path is nested.
		if nestedFields, ok := state.NestedFields[collection]; ok {
			if _, ok := nestedFields.(map[string]string)[joinedPath]; ok {
				nestedPath = nestedPath + "." + field
			}
		}
	}

	return joinedPath, nestedPath
}

// prepareNestedQuery creates a Elasticsearch's nested query based on field_path.
func prepareNestedQuery(
	state *types.State,
	operator string,
	value map[string]interface{},
	fieldName string,
	nestedLevel int,
	collection string,
) map[string]interface{} {
	// Create the innermost query
	operators := strings.Split(operator, ".")
	query := value
	// Iterate over the operators in reverse order
	for i := len(operators) - 1; i >= 0; i-- {
		// Wrap the current query part inside the new level
		query = map[string]interface{}{
			operators[i]: query,
		}
	}
	// Iterate over the fieldPath in to build the nested structure
	pathIdx := strings.LastIndex(fieldName, ".")
	for i := 0; i <= nestedLevel-1; i++ {
		// Check if the current field is nested
		if nestedFields, ok := state.NestedFields[collection]; ok {
			if _, ok := nestedFields.(map[string]string)[fieldName[:pathIdx]]; ok {
				// Create the nested query with the current path and query
				query = map[string]interface{}{
					"nested": map[string]interface{}{
						"path":  fieldName[:pathIdx],
						"query": query,
					},
				}
			}
		}
		// Update the pathIdx to the next level
		pathIdx = strings.LastIndex(fieldName[:pathIdx], ".")
	}

	return query
}

// getCorrectFieldForOperator returns the best field or the `field.subtype` for the given operator.
func getCorrectFieldForOperator(fieldPath, fieldType string, fieldSubTypes map[string]string, bestTypesFamily map[string]bool) (bestField string, typeFound bool) {
	if bestTypesFamily[fieldType] {
		// if the field type is in the best types family, return the field path
		return fieldPath, true
	} else if len(fieldSubTypes) == 0 {
		// if the field has no subtypes, return the field path
		return fieldPath, false
	} else if len(fieldSubTypes) > 0 {
		// if the field has subtypes, return the first matching subfield appended to field path
		for subType, subField := range fieldSubTypes {
			if bestTypesFamily[subType] {
				return fmt.Sprintf("%s.%s", fieldPath, subField), true
			}
		}
	}
	// nothing found, return the field path
	return fieldPath, false
}

// evalComparisonValue evaluates the comparison value for scalar and variable type.
func evalComparisonValue(comparisonValue schema.ComparisonValue, columnName string, operator string) (map[string]interface{}, error) {
	switch compValue := comparisonValue.Interface().(type) {
	case *schema.ComparisonValueScalar:
		if operator == "range" {
			validValue, err := processRangeValue(compValue.Value)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{columnName: validValue}, nil
		}
		return map[string]interface{}{columnName: compValue.Value}, nil
	case *schema.ComparisonValueVariable:
		return map[string]interface{}{columnName: types.Variable(compValue.Name)}, nil
	default:
		return nil, schema.UnprocessableContentError("invalid type of comparison value", map[string]any{
			"value": comparisonValue["type"],
		})
	}
}

// processRangeValue processes the range value for a range comparison.
// It checks if the range value is valid and returns the valid range value.
// If the range value is invalid, it returns an error.
func processRangeValue(rangeValue interface{}) (map[string]interface{}, error) {
	if rangeValue == nil {
		return nil, schema.UnprocessableContentError("invalid range value", nil)
	}

	rangeMap, ok := rangeValue.(map[string]interface{})
	if !ok {
		return nil, schema.UnprocessableContentError("invalid range value", nil)
	}

	// Remove empty range values
	for key, value := range rangeMap {
		if valueStr, ok := value.(string); ok && valueStr == "" {
			delete(rangeMap, key)
		}
	}

	return rangeMap, nil
}
