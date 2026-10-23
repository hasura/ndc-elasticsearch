package connector

import (
	"fmt"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/v2/schema"
)

// evalArgument evaluates the argument and returns its value.
func evalArgument(argument *schema.Argument) (any, error) {
	argumentType, err := argument.Type()
	if err != nil {
		return nil, schema.UnprocessableContentError(fmt.Sprintf("invalid argument: %s", err), nil)
	}

	switch argumentType {
	case schema.ArgumentTypeVariable:
		// If the type is a variable, create a types.Variable object with the
		// argument name and return it.
		variable, err := argument.AsVariable()
		if err != nil {
			return nil, schema.UnprocessableContentError(fmt.Sprintf("invalid variable argument: %s", err), nil)
		}
		return types.Variable(variable.Name), nil
	case schema.ArgumentTypeLiteral:
		literal, err := argument.AsLiteral()
		if err != nil {
			return nil, schema.UnprocessableContentError(fmt.Sprintf("invalid literal argument: %s", err), nil)
		}
		return literal.Value, nil
	default:
		return nil, schema.UnprocessableContentError(fmt.Sprintf("invalid argument type: %s", argumentType), nil)
	}
}

// validateArguments validates that all arguments have been provided, and that no arguments have been given that do not
// map collection's defined arguments.
func validateArguments(params map[string]interface{}, args map[string]schema.Argument) error {
	// Check for missing arguments
	missing := findMissingArgs(params, args)
	if len(missing) > 0 {
		return schema.UnprocessableContentError(fmt.Sprintf("missing arguments: %s", missing), nil)
	}

	// Check for excess arguments
	excess := findExcessArgs(params, args)
	if len(excess) > 0 {
		return schema.UnprocessableContentError(fmt.Sprintf("excess arguments: %s", excess), nil)
	}

	return nil
}

// findMissingArgs returns a slice of argument names that are required by the
// collection but not present in the QueryRequest.
func findMissingArgs(params map[string]interface{}, args map[string]schema.Argument) []string {
	missing := []string{}

	for param := range params {
		// Check if the parameter is not present in the QueryRequest
		if _, ok := args[param]; !ok {
			missing = append(missing, param)
		}
	}

	return missing
}

// findExcessArgs returns a slice of argument names that are present in the
// QueryRequst but not defined in the collection.
func findExcessArgs(parameters map[string]interface{}, arguments map[string]schema.Argument) []string {
	excessArgs := []string{}

	for argName := range arguments {
		// check if the argument name is not defined in the collection
		if _, ok := parameters[argName]; !ok {
			excessArgs = append(excessArgs, argName)
		}
	}

	return excessArgs
}
