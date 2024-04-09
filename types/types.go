package types

import (
	"github.com/hasura/ndc-elasticsearch/elasticsearch"
	"github.com/hasura/ndc-sdk-go/connector"
)

// State is the global state which is shared for every connector request.
type State struct {
	*connector.TelemetryState
	Client *elasticsearch.Client
}

// Configuration contains required settings for the connector.
type Configuration map[string]interface{}
