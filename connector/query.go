package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/hasura/ndc-elasticsearch/elasticsearch"
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

// executeQuery prepares equivalent elasticsearch query, executes it and returns the ndc response.
func executeQuery(ctx context.Context, state *types.State, request *schema.QueryRequest, span trace.Span) (schema.QueryResponse, error) {

	// uncomment to pretty print the query as JSON
	// requestJson, err := json.MarshalIndent(request, "", "  ")
	// if err != nil {
	// 	fmt.Printf("Error marshalling request to JSON: %v\n", err)
	// } else {
	// 	fmt.Printf("request: %s\n\n", string(requestJson))
	// }

	// Set the postProcessor in ctx
	ctx = context.WithValue(ctx, "postProcessor", &types.PostProcessor{})
	logger := connector.GetLogger(ctx)
	rowSets := make([]schema.RowSet, 0)
	index := request.Collection

	// Identify the index from configuration
	nativeQueries := state.Configuration.Queries
	queryConfig, ok := nativeQueries[request.Collection]
	if ok {
		index = queryConfig.Index
	}

	// Prepare the elasticsearch query
	prepareContext, prepareSpan := state.Tracer.Start(ctx, "prepare_elasticsearch_query")
	defer prepareSpan.End()

	dslQuery, err := prepareElasticsearchQuery(prepareContext, request, state, index)
	if err != nil {
		prepareSpan.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	prepareSpan.End()

	// Handle native queries if present
	if ok {
		dslQuery, err = handleNativeQuery(ctx, queryConfig, dslQuery, request.Arguments)
		if err != nil {
			return nil, err
		}
	}

	// Prepare query with variables if present
	if len(request.Variables) != 0 {
		_, variableSpan := state.Tracer.Start(ctx, "prepare_query_with_variables")
		defer variableSpan.End()

		addSpanEvent(variableSpan, logger, "prepare_query_with_variables", map[string]any{
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
	res, err := state.Client.Search(searchContext, index, dslQuery)
	if err != nil {
		searchSpan.SetStatus(codes.Error, err.Error())
		return nil, schema.UnprocessableContentError("failed to execute query", map[string]any{
			"error": err.Error(),
		})
	}
	searchSpan.End()

	// Prepare response based on variables
	if len(request.Variables) != 0 {
		responseContext, responseSpan := state.Tracer.Start(ctx, "prepare_ndc_response")
		defer responseSpan.End()

		addSpanEvent(responseSpan, logger, "prepare_ndc_response", map[string]any{
			"elasticsearch_response": res,
		})
		rowSets = prepareResponseWithVariables(responseContext, res)
	} else {
		responseContext, responseSpan := state.Tracer.Start(ctx, "prepare_ndc_response")
		defer responseSpan.End()

		addSpanEvent(responseSpan, logger, "prepare_ndc_response", map[string]any{
			"elasticsearch_response": res,
		})
		result := prepareResponse(responseContext, res)
		rowSets = append(rowSets, *result)
		responseSpan.End()
	}
	return rowSets, nil
}

// prepareElasticsearchQuery prepares an Elasticsearch query based on the provided query request.
func prepareElasticsearchQuery(ctx context.Context, request *schema.QueryRequest, state *types.State, index string) (map[string]interface{}, error) {
	// Set the user configured default result size in ctx
	ctx = context.WithValue(ctx, elasticsearch.DEFAULT_RESULT_SIZE_KEY, elasticsearch.GetDefaultResultSize())

	query := map[string]interface{}{
		"_source": map[string]interface{}{
			"excludes": []string{"*"},
		},
	}

	span := trace.SpanFromContext(ctx)

	span.AddEvent("prepare_select_query")
	// Select the fields
	if len(request.Query.Fields) != 0 {
		postProcessor := ctx.Value("postProcessor").(*types.PostProcessor)
		postProcessor.IsFields = true
		source, selectedFields, err := prepareSelectFields(ctx, request.Query.Fields, postProcessor, "")
		if err != nil {
			return nil, err
		}
		postProcessor.SelectedFields = selectedFields
		query["_source"] = source
	}

	span.AddEvent("prepare_paginate_query")
	// Set the limit
	if request.Query.Limit != nil {
		query["size"] = *request.Query.Limit
	} else {
		query["size"] = ctx.Value(elasticsearch.DEFAULT_RESULT_SIZE_KEY).(int)
	}

	// Set the offset
	if request.Query.Offset != nil {
		query["from"] = *request.Query.Offset
	}

	// Check if the request has collection arguments
	if hasCollectionArguments(request) {
		// Arguments
		args := handleCollectionArguments(request.Arguments)
		maps.Copy(query, args)
	}

	span.AddEvent("prepare_sort_query")
	// Order by
	if request.Query.OrderBy != nil && len(request.Query.OrderBy.Elements) != 0 {
		sort, err := prepareSortQuery(request.Query.OrderBy, state, index)
		if err != nil {
			return nil, err
		}
		query["sort"] = sort
	}

	span.AddEvent("prepare_aggregate_query")
	// Aggregations
	if request.Query.Aggregates != nil {
		aggs, err := prepareAggregateQuery(ctx, request.Query.Aggregates, state, index)
		if err != nil {
			return nil, err
		}
		if len(aggs) != 0 {
			query["aggs"] = aggs
		}

		// set query size to 0 if aggregation is present
		// this is because, by default, an aggregation query returns both the aggregation result and the hit documents
		// we only want the aggregation result
		// more reading: https://www.elastic.co/guide/en/elasticsearch/reference/current/search-aggregations.html#return-only-agg-results
		query["size"] = 0

		// set track_total_hits to true if aggregation is present
		// this is because, by default, a count query will only return 10,000 hits
		query["track_total_hits"] = true
	}

	span.AddEvent("prepare_filter_query")
	// Filter
	if request.Query.Predicate != nil {
		filter, err := prepareFilterQuery(request.Query.Predicate, state, index)
		if err != nil {
			return nil, err
		}
		if len(filter) != 0 {
			query["query"] = filter
		}
	}

	// Pretty print the query
	queryJSON, _ := json.MarshalIndent(query, "", "  ")
	fmt.Println(string(queryJSON))

	return query, nil
}

// check whether the request has collection arguments
func hasCollectionArguments(request *schema.QueryRequest) bool {
	return len(request.Arguments) != 0
}

func handleCollectionArguments(arguments map[string]schema.Argument) map[string]interface{} {
	query := map[string]interface{}{}

	// Handle search_after
	if arg, ok := arguments["search_after"]; ok {
		query["search_after"] = arg.Value

		// TODO: disable track_total_hits speeds up the query
		// https://www.elastic.co/guide/en/elasticsearch/reference/current/paginate-search-results.html
		// query["track_total_hits"] = false
	}
	return query
}
