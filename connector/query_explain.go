package connector

import (
	"context"
	"encoding/json"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/connector"
	"github.com/hasura/ndc-sdk-go/schema"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (c *Connector) QueryExplain(ctx context.Context, configuration *types.Configuration, state *types.State, request *schema.QueryRequest) (*schema.ExplainResponse, error) {
	span := trace.SpanFromContext(ctx)
	response, err := executeQueryExplainRequest(ctx, state, request, span)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return response, nil
}

func executeQueryExplainRequest(ctx context.Context, state *types.State, request *schema.QueryRequest, span trace.Span) (*schema.ExplainResponse, error) {
	ctx = context.WithValue(ctx, "postProcessor", &types.PostProcessor{})
	logger := connector.GetLogger(ctx)
	index := request.Collection

	prepareContext, prepareSpan := state.Tracer.Start(ctx, "prepare_elasticsearch_query_explain")
	defer prepareSpan.End()

	dslQuery, err := prepareElasticsearchQuery(prepareContext, request, state, index)
	if err != nil {
		prepareSpan.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Prepare query with variables if present
	if len(request.Variables) != 0 {
		_, variableSpan := state.Tracer.Start(ctx, "prepare_query_explain_with_variables")
		defer variableSpan.End()

		addSpanEvent(variableSpan, logger, "prepare_query_explain_with_variables", map[string]any{
			"variables": request.Variables,
		})
		dslQuery, err = executeQueryWithVariables(request.Variables, dslQuery)
		if err != nil {
			variableSpan.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		variableSpan.End()
	}

	// Execute the elasticsearch query
	searchContext, searchSpan := state.Tracer.Start(ctx, "database_request")
	defer searchSpan.End()

	queryJson, _ := json.Marshal(dslQuery)
	setDatabaseAttribute(span, state, index, string(queryJson))
	addSpanEvent(searchSpan, logger, "search_elasticsearch", map[string]any{
		"elasticsearch_request": dslQuery,
	})
	res, err := state.Client.ExplainSearch(searchContext, index, dslQuery)

	if err != nil {
		searchSpan.SetStatus(codes.Error, err.Error())
		return nil, schema.UnprocessableContentError("failed to execute query", map[string]any{
			"error": err.Error(),
		})
	}
	searchSpan.End()

	prettyQueryJson, err := json.MarshalIndent(dslQuery, "", "  ")
	if err != nil {
		return nil, schema.UnprocessableContentError("failed to marshal query to JSON", map[string]any{
			"error": err.Error(),
		})
	}

	prettyResponseJson, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return nil, schema.UnprocessableContentError("failed to marshal response to JSON", map[string]any{
			"error": err.Error(),
		})
	}

	return &schema.ExplainResponse{
		Details: schema.ExplainResponseDetails{
			"query_profile": string(prettyResponseJson),
			"query":         string(prettyQueryJson),
		},
	}, nil
}
