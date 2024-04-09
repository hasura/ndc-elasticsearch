package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/schema"
)

func (c *Connector) Query(ctx context.Context, configuration *types.Configuration, state *types.State, request *schema.QueryRequest) (schema.QueryResponse, error) {
	return nil, schema.NotSupportedError("query has not been supported yet", nil)
}
