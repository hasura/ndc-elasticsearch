package connector

import (
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

// prepareSortQuery prepares the sort query.
func prepareSortQuery(orderBy *schema.OrderBy, state *types.State) ([]map[string]interface{}, error) {
	sort := make([]map[string]interface{}, len(orderBy.Elements))
	for i, element := range orderBy.Elements {
		field := element.Target["name"].(string)
		if _, ok := state.SupportedSortFields[field]; !ok {
			return nil, schema.BadRequestError("sorting not supported on this field", map[string]interface{}{"value": field})
		}
		sort[i] = map[string]interface{}{
			state.SupportedSortFields[field]: map[string]interface{}{"order": element.OrderDirection},
		}
	}
	return sort, nil
}
