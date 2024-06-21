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
type Configuration struct {
	Indices map[string]interface{} `json:"indices"`
	Queries map[string]NativeQuery `json:"queries"`
}

// NativeQuery contains the definition of the native query.
type NativeQuery struct {
	DSL        DSL                     `json:"dsl"`
	Index      string                  `json:"index"`
	ReturnType *ReturnType             `json:"return_type,omitempty"`
	Arguments  *map[string]interface{} `json:"arguments,omitempty"`
}

// DSL contains the dsl query of the native query.
type DSL struct {
	File     *string                 `json:"file,omitempty"`
	Internal *map[string]interface{} `json:"internal,omitempty"`
}

// ReturnType contains the return type of the native query.
type ReturnType struct {
	Kind     string                  `json:"kind"`
	Mappings *map[string]interface{} `json:"mappings,omitempty"`
}

// PostProcessor is used to post process the query response.
type PostProcessor struct {
	IsFields        bool
	StarAggregates  string
	ColumnAggregate map[string]bool
	IsIDSelected    bool
	SelectedFields  map[string]Field
}

// Field is used to represent a field in the query response.
type Field struct {
	Name   string
	Fields map[string]Field
}

type Variable string
