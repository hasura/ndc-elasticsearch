package connector

import (
	"context"

	"github.com/hasura/ndc-elasticsearch/elasticsearch"
	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/connector"
	"github.com/hasura/ndc-sdk-go/schema"
	"go.opentelemetry.io/otel/codes"
)

var configFileName = "configuration.json"

// Connector implements the SDK interface of NDC specification
type Connector struct{}

// TryInitState initializes the connector's in-memory state.
func (c *Connector) TryInitState(ctx context.Context, configuration *types.Configuration, metrics *connector.TelemetryState) (*types.State, error) {
	connectionContext, connectionSpan := metrics.Tracer.Start(ctx, "connect_to_database")
	defer connectionSpan.End()

	client, err := elasticsearch.NewClient(connectionContext)
	if err != nil {
		connectionSpan.RecordError(err)
		connectionSpan.SetStatus(codes.Error, "failed to connect to elasticsearch")
		return nil, err
	}
	elasticsearchInfo, err := client.GetInfo(ctx)
	if err != nil {
		connectionSpan.RecordError(err)
		connectionSpan.SetStatus(codes.Error, "failed to get elasticsearch info")
		return nil, err
	}

	state := &types.State{
		TelemetryState:           metrics,
		Client:                   client,
		SupportedSortFields:      make(map[string]interface{}),
		SupportedAggregateFields: make(map[string]interface{}),
		SupportedFilterFields:    make(map[string]interface{}),
		NestedFields:             make(map[string]interface{}),
		ElasticsearchInfo:        elasticsearchInfo.(map[string]interface{}),
		Configuration:            configuration,
	}

	schema := ParseConfigurationToSchema(configuration, state)

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
		Version: "0.1.6",
		Capabilities: schema.Capabilities{
			Query: schema.QueryCapabilities{
				Variables:  schema.LeafCapability{},
				Aggregates: schema.LeafCapability{},
				Explain:  schema.LeafCapability{},
				NestedFields: schema.NestedFieldCapabilities{
					OrderBy:    schema.LeafCapability{},
					FilterBy:   schema.LeafCapability{},
					Aggregates: schema.LeafCapability{},
				},
				Exists: schema.ExistsCapabilities{
					NestedCollections: schema.LeafCapability{},
				},
			},
		},
	}
}

// MutationExplain explains a mutation by creating an execution plan.
func (c *Connector) MutationExplain(ctx context.Context, configuration *types.Configuration, state *types.State, request *schema.MutationRequest) (*schema.ExplainResponse, error) {
	return nil, schema.NotSupportedError("mutation explain has not been supported yet", nil)
}

// Mutation executes a mutation request.
func (c *Connector) Mutation(ctx context.Context, configuration *types.Configuration, state *types.State, request *schema.MutationRequest) (*schema.MutationResponse, error) {
	return nil, schema.NotSupportedError("mutation has not been supported yet", nil)
}
