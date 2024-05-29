package connector

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/hasura/ndc-elasticsearch/elasticsearch"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/connector"
	"github.com/hasura/ndc-sdk-go/schema"
)

var configFileName = "configuration.json"

// Connector implements the SDK interface of NDC specification
type Connector struct{}

// ParseConfiguration parses the connector's configuration
func (c *Connector) ParseConfiguration(ctx context.Context, configurationDir string) (*types.Configuration, error) {

	configFilePath := filepath.Join(configurationDir, configFileName)

	config, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	var configuration types.Configuration
	err = json.Unmarshal(config, &configuration)
	if err != nil {
		return nil, err
	}

	return &configuration, nil
}

// TryInitState initializes the connector's in-memory state.
func (c *Connector) TryInitState(ctx context.Context, configuration *types.Configuration, metrics *connector.TelemetryState) (*types.State, error) {
	client, err := elasticsearch.NewClient()
	if err != nil {
		return nil, err
	}
	elasticsearchInfo, err := client.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	state := &types.State{
		TelemetryState:           metrics,
		Client:                   client,
		SupportedSortFields:      make(map[string]interface{}),
		SupportedAggregateFields: make(map[string]interface{}),
		SupportedFilterFields:    make(map[string]interface{}),
		ElasticsearchInfo:        elasticsearchInfo.(map[string]interface{}),
	}

	schema := parseConfigurationToSchema(configuration, state)

	state.Schema = schema
	return state, nil
}

// HealthCheck checks the health of the connector.
func (c *Connector) HealthCheck(ctx context.Context, configuration *types.Configuration, state *types.State) error {
	if err := state.Client.Ping(); err != nil {
		return err
	}
	return nil
}

// GetCapabilities get the connector's capabilities.
func (c *Connector) GetCapabilities(configuration *types.Configuration) schema.CapabilitiesResponseMarshaler {
	return &schema.CapabilitiesResponse{
		Version: "0.1.2",
		Capabilities: schema.Capabilities{
			Query: schema.QueryCapabilities{
				Variables:  schema.LeafCapability{},
				Aggregates: schema.LeafCapability{},
			},
		},
	}
}

// QueryExplain explains a query by creating an execution plan.
func (c *Connector) QueryExplain(ctx context.Context, configuration *types.Configuration, state *types.State, request *schema.QueryRequest) (*schema.ExplainResponse, error) {
	return nil, schema.NotSupportedError("query explain has not been supported yet", nil)
}

// MutationExplain explains a mutation by creating an execution plan.
func (c *Connector) MutationExplain(ctx context.Context, configuration *types.Configuration, state *types.State, request *schema.MutationRequest) (*schema.ExplainResponse, error) {
	return nil, schema.NotSupportedError("mutation explain has not been supported yet", nil)
}

// Mutation executes a mutation request.
func (c *Connector) Mutation(ctx context.Context, configuration *types.Configuration, state *types.State, request *schema.MutationRequest) (*schema.MutationResponse, error) {
	return nil, schema.NotSupportedError("mutation has not been supported yet", nil)
}
