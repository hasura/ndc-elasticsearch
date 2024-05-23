package types

import (
	"github.com/hasura/ndc-elasticsearch/elasticsearch"
	"github.com/hasura/ndc-sdk-go/connector"
)

// State is the global state which is shared for every connector request.
type State struct {
	*connector.TelemetryState
	Client                   *elasticsearch.Client
	SupportedSortFields      map[string]string
	SupportedAggregateFields map[string]string
	SupportedFilterFields    map[string]interface{}
	ElasticsearchInfo        map[string]interface{}
}

// Configuration contains required settings for the connector.
type Configuration map[string]interface{}

// PostProcessor is used to post process the query response.
type PostProcessor struct {
	IsFields        bool
	StarAggregates  string
	ColumnAggregate []string
	IsIDSelected    bool
	SelectedFields  map[string]string
}

type Variable string
