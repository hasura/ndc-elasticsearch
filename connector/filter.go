package connector

import (
	"strings"

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
				filter = joinNestedFieldPath(state, "bool.must_not.exists", value, fieldName, len(expr.Column.FieldPath), collection)
			}
		}
		return filter, nil
	}
	return nil, schema.UnprocessableContentError("invalid unary comparison operator", map[string]any{
		"operator": expr.Operator,
	})
}

// handleExpressionBinaryComparisonOperator processes the binary comparison operator expression.
func handleExpressionBinaryComparisonOperator(expr *schema.ExpressionBinaryComparisonOperator, state *types.State, collection string) (map[string]interface{}, error) {
	var filter map[string]interface{}
	fieldName, _ := joinFieldPath(state, expr.Column.FieldPath, expr.Column.Name, collection)
	var bestSubField string
	switch expr.Operator {
	case "match", "match_phrase", "match_phrase_prefix", "match_bool_prefix":
		bestSubField = getTextFieldFromState(state, fieldName, collection)
	case "term", "prefix", "terms":
		bestSubField = getKeywordFieldFromState(state, fieldName, collection)
	case "wildcard", "regexp":
		bestSubField = getWildcardFieldFromState(state, fieldName, collection)
	default:
		return nil, schema.UnprocessableContentError("invalid binary comaparison operator", map[string]any{
			"expression": expr.Operator,
		})
	}

	value, err := evalComparisonValue(expr.Value, bestSubField)
	if err != nil {
		return nil, err
	}
	filter = map[string]interface{}{
		expr.Operator: value,
	}
	if nestedFields, ok := state.NestedFields[collection]; ok {
		if _, ok := nestedFields.(map[string]string)[expr.Column.Name]; ok {
			filter = joinNestedFieldPath(state, expr.Operator, value, fieldName, len(expr.Column.FieldPath), collection)
		}
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

// joinNestedFieldPath creates a Elasticsearch's nested query based on field_path.
func joinNestedFieldPath(state *types.State, operator string, value map[string]interface{}, fieldName string, nestedLevel int, collection string) map[string]interface{} {
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

// getKeywordFieldFromState retrieves the best matching field for term level queries
// from the state. If the field is found, it returns the corresponding field
// name; otherwise, it returns the original columnName.
func getKeywordFieldFromState(state *types.State, columnName string, collection string) string {
	if collectionFields, ok := state.SupportedFilterFields[collection]; ok {
		if keywordFields, ok := collectionFields.(map[string]interface{})["term_level_queries"].(map[string]string); ok {
			if keywordField, ok := keywordFields[columnName]; ok {
				return keywordField
			}
		}
		if wildcardFields, ok := collectionFields.(map[string]interface{})["unstructured_text"].(map[string]string); ok {
			if wildcardField, ok := wildcardFields[columnName]; ok {
				return wildcardField
			}
		}
	}
	return columnName
}

// getTextFieldFromState retrieves the best matching field for full text queries
// from the state. If the field is found, it returns the corresponding field
// name; otherwise, it returns the original columnName.
func getTextFieldFromState(state *types.State, columnName string, collection string) string {
	if collectionField, ok := state.SupportedFilterFields[collection]; ok {
		if textFields, ok := collectionField.(map[string]interface{})["full_text_queries"].(map[string]string); ok {
			if textField, ok := textFields[columnName]; ok {
				return textField
			}
		}
	}
	return columnName
}

// getWildcardFieldFromState retrieves the best matching field for wildcard and regexp
// queries from the state. If the field is found, it returns the corresponding field
// name; otherwise, it returns the original columnName.
func getWildcardFieldFromState(state *types.State, columnName string, collection string) string {
	if collectionFields, ok := state.SupportedFilterFields[collection]; ok {
		if wildcardFields, ok := collectionFields.(map[string]interface{})["unstructured_text"].(map[string]string); ok {
			if wildcardField, ok := wildcardFields[columnName]; ok {
				return wildcardField
			}
		}
		if keywordFields, ok := collectionFields.(map[string]interface{})["term_level_queries"].(map[string]string); ok {
			if keywordField, ok := keywordFields[columnName]; ok {
				return keywordField
			}
		}
	}
	return columnName
}

// evalComparisonValue evaluates the comparison value for scalar and variable type.
func evalComparisonValue(comparisonValue schema.ComparisonValue, columnName string) (map[string]interface{}, error) {
	switch compValue := comparisonValue.Interface().(type) {
	case *schema.ComparisonValueScalar:
		return map[string]interface{}{
			columnName: compValue.Value,
		}, nil
	case *schema.ComparisonValueVariable:
		return map[string]interface{}{
			columnName: types.Variable(compValue.Name),
		}, nil
	default:
		return nil, schema.UnprocessableContentError("invalid type of comparison value", map[string]any{
			"value": comparisonValue["type"],
		})
	}
}
