package connector

import (
	"github.com/hasura/ndc-elasticsearch/internal"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// prepareSortQuery prepares the sort query.
func prepareSortQuery(orderBy *schema.OrderBy, state *types.State, collection string) ([]map[string]interface{}, error) {
	sort := make([]map[string]interface{}, len(orderBy.Elements))
	for i, element := range orderBy.Elements {
		sortElmnt, err := prepareSortElement(&element, state, collection)
		if err != nil {
			return nil, err
		}
		sort[i] = sortElmnt
	}
	return sort, nil
}

// prepareSortElement prepares the sort element for Elasticsearch query.
//
// It takes in the OrderByElement, state, and collection as parameters.
// It returns the prepared sort element and an error if any.
func prepareSortElement(element *schema.OrderByElement, state *types.State, collection string) (map[string]interface{}, error) {
	sort := make(map[string]interface{})
	switch target := element.Target.Interface().(type) {
	case *schema.OrderByColumn:
		// Join the field path to get the field path and nested path.
		fieldPath, nestedPath := joinFieldPath(state, target.FieldPath, target.Name, collection)

		validField := internal.ValidateFieldOperation(state, "sort", collection, fieldPath)
		if validField == "" {
			return nil, schema.UnprocessableContentError("sorting not supported on this field", map[string]any{
				"value": fieldPath,
			})
		}
		fieldPath = validField
		sort[fieldPath] = map[string]interface{}{
			"order": string(element.OrderDirection),
		}

		// Check if the field is nested.
		if nestedPath != "" {
			// If the field is nested, add the nested path to the sort element.
			sort[fieldPath] = map[string]interface{}{
				"nested": map[string]interface{}{
					"path": nestedPath,
				},
				"order": string(element.OrderDirection),
			}
		}
	default:
		return nil, schema.UnprocessableContentError("invalid order by field", map[string]any{
			"value": element.Target,
		})
	}

	return sort, nil
}
