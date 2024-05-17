package connector

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/hasura/ndc-elasticsearch/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// JSONAttribute creates a OpenTelemetry attribute with JSON data
func JSONAttribute(key string, data any) attribute.KeyValue {
	switch d := data.(type) {
	case json.RawMessage:
		return attribute.String(key, string(d))
	default:
		jsonBytes, _ := json.Marshal(data)
		return attribute.String(key, string(jsonBytes))
	}
}

// DebugJSONAttributes create OpenTelemetry attributes with JSON data.
// They are only visible on debug mode
func DebugJSONAttributes(data map[string]any, isDebug bool) []attribute.KeyValue {
	if !isDebug || len(data) == 0 {
		return []attribute.KeyValue{}
	}

	attrs := []attribute.KeyValue{}
	for k, v := range data {
		attrs = append(attrs, JSONAttribute(k, v))
	}
	return attrs
}

// AddSpanEvent adds an event to the span with the given name and data.
func addSpanEvent(span trace.Span, logger *slog.Logger, name string, data map[string]any, options ...trace.EventOption) {
	logger.Debug(name, slog.Any("data", data))
	attrs := DebugJSONAttributes(data, isDebug(logger))
	span.AddEvent(name, append(options, trace.WithAttributes(attrs...))...)
}

// isDebug returns true if the logger is in debug mode
func isDebug(logger *slog.Logger) bool {
	return logger.Enabled(context.TODO(), slog.LevelDebug)
}

func setDatabaseAttribute(span trace.Span, state *types.State, index string, query string) {
	elasticsearchInfo := state.ElasticsearchInfo

	if clusterName, ok := elasticsearchInfo["cluster_name"]; ok {
		span.SetAttributes(attribute.String("db.elasticsearch.cluster.name", clusterName.(string)))
	}
	if instanceID, ok := elasticsearchInfo["name"]; ok {
		span.SetAttributes(attribute.String("db.instance.id", instanceID.(string)))
	}
	span.SetAttributes(
		attribute.String("db.operation", "search"),
		attribute.String("http.request.method", "POST"),
		attribute.String("db.elasticsearch.path_parts.index", index),
		attribute.String("db.statement", query),
		attribute.String("db.system", "elasticsearch"),
	)
}
