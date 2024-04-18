package connector

import "github.com/hasura/ndc-sdk-go/schema"

func prepareFilterQuery(expression schema.Expression) (map[string]interface{}, error) {
	filter := make(map[string]interface{})
	switch expr := expression.Interface().(type) {
	case *schema.ExpressionBinaryComparisonOperator:
		switch expr.Operator {
		case "match", "match_phrase", "match_phrase_prefix", "match_bool_prefix", "term", "prefix", "wildcard", "regexp", "terms":
			value, err := evalElasticComparisonValue(expr.Value)
			if err != nil {
				return nil, err
			}
			filter[expr.Operator] = map[string]interface{}{
				expr.Column.Name: value,
			}
			return filter, nil
		default:
			return nil, schema.UnprocessableContentError("invalid filter", map[string]any{
				"expression": expression,
			})
		}
	case *schema.ExpressionAnd:
		expressionAnd, err := expr.Expressions[0].AsAnd()
		if err != nil {
			return nil, err
		}
		queries := make([]map[string]interface{}, 0)
		for _, expr := range expressionAnd.Expressions {
			res, err := prepareFilterQuery(expr)
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
		expressionAnd, err := expr.Expressions[0].AsAnd()
		if err != nil {
			return nil, err
		}
		queries := make([]map[string]interface{}, 0)
		for _, expr := range expressionAnd.Expressions {
			res, err := prepareFilterQuery(expr)
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
		expressionAnd, err := expr.Expression.AsAnd()
		if err != nil {
			return nil, err
		}
		queries := make([]map[string]interface{}, 0)
		for _, expr := range expressionAnd.Expressions {
			res, err := prepareFilterQuery(expr)
			if err != nil {
				return nil, err
			}
			queries = append(queries, res)
		}

		filter["bool"] = map[string]interface{}{
			"must_not": queries,
		}
		return filter, nil
	default:
		return nil, schema.UnprocessableContentError("invalid filter type", map[string]any{
			"expression": expression,
		})
	}
}

func evalElasticComparisonValue(comparisonValue schema.ComparisonValue) (any, error) {
	switch compValue := comparisonValue.Interface().(type) {
	case *schema.ComparisonValueScalar:
		return compValue.Value, nil
	default:
		return nil, schema.UnprocessableContentError("invalid comparison value", map[string]any{
			"value": comparisonValue,
		})
	}
}
