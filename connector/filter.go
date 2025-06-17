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
	columnPath, predicate := getPredicate(expression)

	switch expr := predicate.Interface().(type) {
	case *schema.ExpressionUnaryComparisonOperator:
		fieldPath := strings.Split(columnPath, ".")
		expr.Column.FieldPath = fieldPath
		return handleExpressionUnaryComparisonOperator(expr, state, collection)
	case *schema.ExpressionBinaryComparisonOperator:
		fieldPath := strings.Split(columnPath, ".")
		expr.Column.FieldPath = fieldPath
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
		return buildOrClauseQuery(expr.Expressions, state, collection)
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

// buildOrClauseQuery constructs an Elasticsearch boolean query with "should" conditions
// from a list of expressions. In Elasticsearch, "should" conditions are equivalent to OR logic.
func buildOrClauseQuery(expressions []schema.Expression, state *types.State, collection string) (map[string]interface{}, error) {
	if isEmptyOrClause(expressions) {
		// an empty `or` clause is equivalent to a simple `false` clause according to the NDC Spec
		// elasiticsearch does not have this behaviour inbuilt
		// it treats an empty `or` clause  as a match all
		// so, we explicitly add a `must_not` condition to negate the match all
		return map[string]interface{}{
			"bool": map[string]interface{}{
				"must_not": map[string]interface{}{
					"match_all": map[string]interface{}{},
				},
			},
		}, nil
	}
	
	queries := make([]map[string]interface{}, 0)
	for _, expr := range expressions {
		res, err := prepareFilterQuery(expr, state, collection)
		if err != nil {
			return nil, err
		}
		queries = append(queries, res)
	}
	
	filter := make(map[string]interface{})
	filter["bool"] = map[string]interface{}{
		"should": queries,
	}
	return filter, nil
}

func isEmptyOrClause(expressions []schema.Expression) bool {
	return len(expressions) == 0
}

// getPredicate checks if a schema.Expression has nested filtering
// if it does, it traverses the schema.Expression recursively until it finds a non-nested query predicate
func getPredicate(expression schema.Expression) (string, schema.Expression) {
	if nested, fieldName := requiresNestedFiltering(expression); nested {
		expressionPredicate, ok := expression["predicate"].(schema.Expression)
		if !ok {
			return "", nil
		}

		columnPathPostfix, predicate := getPredicate(expressionPredicate)
		return fmt.Sprintf("%s.%s", fieldName, columnPathPostfix), predicate
	}
	switch expr := expression.Interface().(type) {
	case *schema.ExpressionUnaryComparisonOperator:
		return expr.Column.Name, expression
	case *schema.ExpressionBinaryComparisonOperator:
		return expr.Column.Name, expression
	}

	return "", expression
}

func requiresNestedFiltering(predicate schema.Expression) (requiresNestedFiltering bool, nestedFieldName string) {
	inCollection, ok := predicate["in_collection"].(schema.ExistsInCollection)
	if !ok {
		return false, ""
	}
	collection, err := inCollection.AsNestedCollection()
	if err != nil {
		return false, ""
	}
	if collection.Type == "nested_collection" {
		return true, collection.ColumnName
	}
	return false, ""
}

// handleExpressionUnaryComparisonOperator processes the unary comparison operator expression.
func handleExpressionUnaryComparisonOperator(expr *schema.ExpressionUnaryComparisonOperator, state *types.State, collection string) (map[string]interface{}, error) {
	if expr.Operator == "is_null" {
		if len(expr.Column.FieldPath) == 0 || expr.Column.FieldPath[len(expr.Column.FieldPath)-1] != expr.Column.Name {
			// if the column name is not the last element in fieldPath, we'll add it so that the fieldpath is complete
			expr.Column.FieldPath = append(expr.Column.FieldPath, expr.Column.Name)
		}

		fieldName := strings.Join(expr.Column.FieldPath, ".")
		value := map[string]interface{}{
			"field": fieldName,
		}
		filter := map[string]interface{}{"bool": map[string]interface{}{"must_not": map[string]interface{}{"exists": value}}}
		var err error
		filter, err = prepareNestedQuery(state, filter, fieldName, collection)
		if err != nil {
			return nil, err
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
	if len(expr.Column.FieldPath) == 0 || expr.Column.FieldPath[len(expr.Column.FieldPath)-1] != expr.Column.Name {
		// if the column name is not the last element in fieldPath, we'll add it so that the fieldpath is complete
		expr.Column.FieldPath = append(expr.Column.FieldPath, expr.Column.Name)
	}

	fieldPath := strings.Join(expr.Column.FieldPath, ".")
	fieldType, fieldSubTypes, _, err := state.Configuration.GetFieldProperties(collection, fieldPath)
	if err != nil {
		return nil, schema.UnprocessableContentError("unable to get field types", map[string]any{
			"fieldPath": fieldPath,
			"index":     collection,
		})
	}

	bestFieldOrSubField, operatorFound := internal.GetBestFieldOrSubFieldForQuery(fieldPath, fieldType, fieldSubTypes, expr.Operator)
	if !operatorFound {
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

	filter, err = prepareNestedQuery(state, filter, fieldPath, collection)

	if err != nil {
		return nil, err
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

func prepareNestedQuery(
	state *types.State,
	filter map[string]interface{},
	fieldPath string,
	collection string,
) (map[string]interface{}, error) {
	isNested, err := state.Configuration.IsFieldNested(collection, fieldPath)
	if err != nil {
		return nil, err
	}
	if isNested {
		var nestedFilter map[string]interface{} = make(map[string]interface{})
		nestedFilter["nested"] = map[string]interface{}{
			"path":  fieldPath,
			"query": filter,
		}

		filter = nestedFilter
	}
	splitFieldPath := strings.Split(fieldPath, ".")
	if len(splitFieldPath) == 1 {
		return filter, nil
	}
	return prepareNestedQuery(state, filter, strings.Join(splitFieldPath[:len(splitFieldPath)-1], "."), collection)
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
