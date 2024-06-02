package types

import (
	"github.com/hasura/ndc-elasticsearch/elasticsearch"
	"github.com/hasura/ndc-sdk-go/connector"
	"github.com/hasura/ndc-sdk-go/schema"
)

// State is the global state which is shared for every connector request.
type State struct {
	*connector.TelemetryState
	Client                   *elasticsearch.Client
	SupportedSortFields      map[string]interface{}
	SupportedAggregateFields map[string]interface{}
	SupportedFilterFields    map[string]interface{}
	ElasticsearchInfo        map[string]interface{}
	Schema                   *schema.SchemaResponse
	NestedFields             map[string]interface{}
}

// Configuration contains required settings for the connector.
type Configuration map[string]interface{}

// PostProcessor is used to post process the query response.
type PostProcessor struct {
	IsFields        bool
	StarAggregates  string
	ColumnAggregate []string
	IsIDSelected    bool
	SelectedFields  map[string]Field
}

type Field struct {
	Name   string
	Fields map[string]Field
}

type Variable string
