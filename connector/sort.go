package connector

import (
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

func prepareSortElement(element *schema.OrderByElement, state *types.State, collection string) (map[string]interface{}, error) {
	sort := make(map[string]interface{})
	switch target := element.Target.Interface().(type) {
	case *schema.OrderByColumn:
		fieldName, fieldPath := joinFieldPath(state, target.FieldPath, target.Name, collection)

		if collectionSortFields, ok := state.SupportedSortFields[collection]; ok {
			if sortField, ok := collectionSortFields.(map[string]string)[fieldName]; ok {
				fieldName = sortField
			} else {
				return nil, schema.UnprocessableContentError("sorting not supported on this field", map[string]any{
					"value": fieldName,
				})
			}
		}
		sort[fieldName] = map[string]interface{}{
			"order": string(element.OrderDirection),
		}
		if nestedFields, ok := state.NestedFields[collection]; ok {
			if _, ok := nestedFields.(map[string]string)[target.Name]; ok {
				sort[fieldName] = map[string]interface{}{
					"nested": map[string]interface{}{
						"path": fieldPath,
					},
					"order": string(element.OrderDirection),
				}
			}
		}
	default:
		return nil, schema.UnprocessableContentError("invalid order by field", map[string]any{
			"value": element.Target,
		})
	}
	return sort, nil
}
