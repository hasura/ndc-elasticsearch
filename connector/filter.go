package connector

import (
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// prepareFilterQuery prepares a filter query based on the given expression.
func prepareFilterQuery(expression schema.Expression, state *types.State) (map[string]interface{}, error) {
	filter := make(map[string]interface{})
	switch expr := expression.Interface().(type) {
	case *schema.ExpressionUnaryComparisonOperator:
		return handleExpressionUnaryComparisonOperator(expr)
	case *schema.ExpressionBinaryComparisonOperator:
		return handleExpressionBinaryComparisonOperator(expr, state)
	case *schema.ExpressionAnd:
		queries := make([]map[string]interface{}, 0)
		for _, expr := range expr.Expressions {
			res, err := prepareFilterQuery(expr, state)
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
			res, err := prepareFilterQuery(expr, state)
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
		res, err := prepareFilterQuery(expr.Expression, state)
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
func handleExpressionUnaryComparisonOperator(expr *schema.ExpressionUnaryComparisonOperator) (map[string]interface{}, error) {
	filter := make(map[string]interface{})
	if expr.Operator == "is_null" {
		filter["bool"] = map[string]interface{}{
			"must_not": []map[string]interface{}{
				{
					"exists": map[string]interface{}{
						"field": expr.Column.Name,
					},
				},
			},
		}
		return filter, nil
	}
	return nil, schema.UnprocessableContentError("invalid unary comparison operator", map[string]any{
		"operator": expr.Operator,
	})
}

// handleExpressionBinaryComparisonOperator processes the binary comparison operator expression.
func handleExpressionBinaryComparisonOperator(expr *schema.ExpressionBinaryComparisonOperator, state *types.State) (map[string]interface{}, error) {
	filter := make(map[string]interface{})
	value, err := evalComparisonValue(expr.Value)
	if err != nil {
		return nil, err
	}
	switch expr.Operator {
	case "match", "match_phrase", "match_phrase_prefix", "match_bool_prefix":
		if textFields, ok := state.SupportedFilterFields["full_text_queries"].(map[string]string); ok {
			if textField, ok := textFields[expr.Column.Name]; ok {
				expr.Column.Name = textField
			}
		}
		filter[expr.Operator] = map[string]interface{}{
			expr.Column.Name: value,
		}
	case "term", "prefix", "terms":
		if keywordFields, ok := state.SupportedFilterFields["term_level_queries"].(map[string]string); ok {
			if keywordField, ok := keywordFields[expr.Column.Name]; ok {
				expr.Column.Name = keywordField
			}
		}
		filter[expr.Operator] = map[string]interface{}{
			expr.Column.Name: value,
		}
	case "wildcard", "regexp":
		if wildcardFields, ok := state.SupportedFilterFields["unstructured_text"].(map[string]string); ok {
			if wildcardField, ok := wildcardFields[expr.Column.Name]; ok {
				expr.Column.Name = wildcardField
			}
		}
		filter[expr.Operator] = map[string]interface{}{
			expr.Column.Name: value,
		}
	default:
		return nil, schema.UnprocessableContentError("invalid binary comaparison operator", map[string]any{
			"expression": expr.Operator,
		})
	}
	return filter, nil
}

// evalComparisonValue evaluates the comparison value for scalar and variable type.
func evalComparisonValue(comparisonValue schema.ComparisonValue) (any, error) {
	switch compValue := comparisonValue.Interface().(type) {
	case *schema.ComparisonValueScalar:
		return compValue.Value, nil
	case *schema.ComparisonValueVariable:
		return types.Variable(compValue.Name), nil
	default:
		return nil, schema.UnprocessableContentError("invalid comparison value", map[string]any{
			"value": comparisonValue,
		})
	}
}
