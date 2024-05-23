package connector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hasura/ndc-elasticsearch/types"
	"github.com/hasura/ndc-sdk-go/connector"
	"github.com/hasura/ndc-sdk-go/schema"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Query executes a query request.
func (c *Connector) Query(ctx context.Context, configuration *types.Configuration, state *types.State, request *schema.QueryRequest) (schema.QueryResponse, error) {
	span := trace.SpanFromContext(ctx)
	response, err := executeQuery(ctx, state, request, span)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return response, nil
}

// executeQuery prepares equivalent elasticsearch query, executes it and returns the ndc response
func executeQuery(ctx context.Context, state *types.State, request *schema.QueryRequest, span trace.Span) (schema.QueryResponse, error) {
	// Set the postProcessor in ctx
	ctx = context.WithValue(ctx, "postProcessor", &types.PostProcessor{})
	logger := connector.GetLogger(ctx)
	rowSets := make([]schema.RowSet, 0)

	prepareContext, prepareSpan := state.Tracer.Start(ctx, "prepare_elasticsearch_query")
	defer prepareSpan.End()

	body, err := prepareElasticsearchQuery(prepareContext, request, state)
	if err != nil {
		prepareSpan.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	prepareSpan.End()

	if request.Variables == nil || len(request.Variables) == 0 {
		searchContext, searchSpan := state.Tracer.Start(ctx, "database_request")
		defer searchSpan.End()

		queryJson, _ := json.Marshal(body)
		setDatabaseAttribute(span, state, request.Collection, string(queryJson))
		addSpanEvent(searchSpan, logger, "search_elasticsearch", map[string]any{
			"elasticsearch_request": body,
		})

		res, err := state.Client.Search(searchContext, request.Collection, body)
		if err != nil {
			searchSpan.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		searchSpan.End()

		responseContext, responseSpan := state.Tracer.Start(ctx, "prepare_ndc_response")
		defer responseSpan.End()

		addSpanEvent(responseSpan, logger, "prepare_ndc_response", map[string]any{
			"elasticsearch_response": res,
		})
		result := prepareResponse(responseContext, res)
		rowSets = append(rowSets, *result)
		responseSpan.End()
	} else {
		_, variableSpan := state.Tracer.Start(ctx, "prepare_query_with_variables")
		defer variableSpan.End()

		addSpanEvent(variableSpan, logger, "prepare_query_with_variables", map[string]any{
			"variables": request.Variables,
		})
		variableQuery, err := executeQueryWithVariables(request.Variables, body)
		if err != nil {
			variableSpan.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		variableSpan.End()

		searchContext, searchSpan := state.Tracer.Start(ctx, "database_request")
		defer searchSpan.End()

		queryJson, _ := json.Marshal(variableQuery)
		setDatabaseAttribute(span, state, request.Collection, string(queryJson))
		addSpanEvent(searchSpan, logger, "search_elasticsearch", map[string]any{
			"elasticsearch_request": variableQuery,
		})
		res, err := state.Client.Search(searchContext, request.Collection, variableQuery)
		if err != nil {
			searchSpan.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		searchSpan.End()

		responseContext, responseSpan := state.Tracer.Start(ctx, "prepare_ndc_response")
		defer responseSpan.End()

		addSpanEvent(responseSpan, logger, "prepare_ndc_response", map[string]any{
			"elasticsearch_response": res,
		})
		rowSets = prepareResponseWithVariables(responseContext, res)
	}
	return rowSets, nil
}

// prepareElasticsearchQuery prepares an Elasticsearch query based on the provided query request.
func prepareElasticsearchQuery(ctx context.Context, request *schema.QueryRequest, state *types.State) (map[string]interface{}, error) {
	query := map[string]interface{}{
		"_source": map[string]interface{}{
			"excludes": []string{"*"},
		},
	}

	span := trace.SpanFromContext(ctx)

	span.AddEvent("prepare_select_query")
	// Select the fields
	if request.Query.Fields != nil {
		fields, err := prepareSelectQuery(ctx, state, request.Query.Fields)
		if err != nil {
			return nil, err
		}
		query["_source"] = fields
	}

	span.AddEvent("prepare_paginate_query")
	// Set the limit
	if request.Query.Limit != nil {
		query["size"] = *request.Query.Limit
	}

	// Set the offset
	if request.Query.Offset != nil {
		query["from"] = *request.Query.Offset
	}

	span.AddEvent("prepare_sort_query")
	// Order by
	if request.Query.OrderBy != nil && len(request.Query.OrderBy.Elements) != 0 {
		sort, err := prepareSortQuery(request.Query.OrderBy, state)
		if err != nil {
			return nil, err
		}
		query["sort"] = sort
	}

	span.AddEvent("prepare_aggregate_query")
	// Aggregations
	if request.Query.Aggregates != nil {
		aggs, err := prepareAggregateQuery(ctx, request.Query.Aggregates, state)
		if err != nil {
			return nil, err
		}
		if len(aggs) != 0 {
			query["aggs"] = aggs
		}
	}

	span.AddEvent("prepare_filter_query")
	// Filter
	if request.Query.Predicate != nil {
		filter, err := prepareFilterQuery(request.Query.Predicate, state)
		if err != nil {
			return nil, err
		}
		if len(filter) != 0 {
			query["query"] = filter
		}
	}

	// Pretty print query
	queryJSON, _ := json.MarshalIndent(query, "", "  ")
	fmt.Println(string(queryJSON))

	return query, nil
}
