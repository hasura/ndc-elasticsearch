package connector

import "github.com/hasura/ndc-sdk-go/schema"

func prepareFilterQuery(expression schema.Expression) (map[string]interface{}, error) {
	filter := make(map[string]interface{})
	switch expr := expression.Interface().(type) {
	case *schema.ExpressionUnaryComparisonOperator:
		switch expr.Operator {
		case "is_null":
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
		default:
			return nil, schema.UnprocessableContentError("invalid unary comparison operator", map[string]any{
				"operator": expr.Operator,
			})
		}
	case *schema.ExpressionBinaryComparisonOperator:
		switch expr.Operator {
		case "match", "match_phrase", "match_phrase_prefix", "match_bool_prefix", "term", "prefix", "wildcard", "regexp", "terms":
			value, err := evalElasticComparisonValue(expr.Value)
			if err != nil {
				return nil, err
			}
			if expr.Operator == "terms" {
				var ok bool
				value, ok = value.([]interface{})
				if !ok {
					return nil, schema.UnprocessableContentError("invalid value for terms operator, expected array", map[string]any{
						"value": value,
					})
				}
			}
			filter[expr.Operator] = map[string]interface{}{
				expr.Column.Name: value,
			}
			return filter, nil
		default:
			return nil, schema.UnprocessableContentError("invalid binary comaparison operator", map[string]any{
				"expression": expr.Operator,
			})
		}
	case *schema.ExpressionAnd:
		queries := make([]map[string]interface{}, 0)
		for _, expr := range expr.Expressions {
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
		queries := make([]map[string]interface{}, 0)
		for _, expr := range expr.Expressions {
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
		res, err := prepareFilterQuery(expr.Expression)
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
