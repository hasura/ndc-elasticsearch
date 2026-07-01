package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// sensitiveSentinel is a stand-in for any PII / sensitive value that an
// Elasticsearch error response might echo back (a field value from the query,
// a document fragment in root_cause.reason, etc). The consumer-facing error
// must NEVER contain it; it may only appear in DEBUG logs.
const sensitiveSentinel = "SENSITIVE_FIELD_VALUE"

// newDebugLogBuffer redirects the default slog logger to a debug-level JSON
// buffer (GetLogger falls back to slog.Default when the context has no logger)
// and returns the buffer plus a restore func.
func newDebugLogBuffer(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return &buf, func() { slog.SetDefault(prev) }
}

// TestParseResponseDoesNotLeakRootCauseReason feeds parseResponse an
// Elasticsearch error body whose root_cause.reason embeds a sensitive sentinel
// and asserts:
//
//   - the returned consumer-facing error does NOT contain the sentinel,
//
//   - the returned error DOES carry the safe category (root_cause.type) and
//     mapped guidance,
//
//   - the full ES error object (with the sentinel) is captured at DEBUG only.
//
//     go test -v -run TestParseResponseDoesNotLeakRootCauseReason ./elasticsearch/
func TestParseResponseDoesNotLeakRootCauseReason(t *testing.T) {
	logBuf, restore := newDebugLogBuffer(t)
	defer restore()

	esError := map[string]interface{}{
		"error": map[string]interface{}{
			"root_cause": []interface{}{
				map[string]interface{}{
					"type":   "security_exception",
					"reason": "action [indices:data/read/search] is unauthorized for user [" + sensitiveSentinel + "]",
				},
			},
			"type":   "security_exception",
			"reason": "action [indices:data/read/search] is unauthorized for user [" + sensitiveSentinel + "]",
		},
		"status": 403,
	}
	bodyBytes, err := json.Marshal(esError)
	if err != nil {
		t.Fatalf("failed to marshal fake es error: %v", err)
	}

	res := &esapi.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(bytes.NewReader(bodyBytes)),
	}

	_, gotErr := parseResponse(context.Background(), res)
	if gotErr == nil {
		t.Fatal("expected an error from parseResponse on an error response, got nil")
	}

	errStr := gotErr.Error()
	if strings.Contains(errStr, sensitiveSentinel) {
		t.Fatalf("consumer-facing error LEAKED the sensitive sentinel: %q", errStr)
	}
	// The safe category and mapped guidance should be present.
	if !strings.Contains(errStr, "security_exception") {
		t.Errorf("expected the safe root_cause.type category in the error, got: %q", errStr)
	}
	if !strings.Contains(errStr, "authorization failed") {
		t.Errorf("expected mapped guidance for security_exception, got: %q", errStr)
	}

	// The sentinel MUST be present in the DEBUG logs (intentional detail).
	logs := logBuf.String()
	if !strings.Contains(logs, sensitiveSentinel) {
		t.Errorf("expected the full ES error object (with sentinel) in DEBUG logs, not found in:\n%s", logs)
	}
}

// newErrorES returns a fake Elasticsearch server that authenticates (HEAD / and
// GET /) successfully but returns a non-401 error response for every _search,
// with a body that embeds the sensitive sentinel inside root_cause.reason.
func newErrorES(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":{"number":"8.0.0"}}`))
		default:
			// Non-401 error response carrying a sensitive value in the body.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"root_cause":[{"type":"x_content_parse_exception","reason":"failed to parse value [` + sensitiveSentinel + `]"}],"type":"x_content_parse_exception","reason":"failed to parse value [` + sensitiveSentinel + `]"},"status":400}`))
		}
	}))
}

// TestSearchErrorResponseDoesNotLeakBody exercises the end-to-end search path
// against a fake cluster that returns a non-401 error response whose body
// contains the sensitive sentinel. It asserts the consumer-facing error does
// NOT contain the raw body / sentinel, while the raw body IS captured at DEBUG.
//
//	go test -v -run TestSearchErrorResponseDoesNotLeakBody ./elasticsearch/
func TestSearchErrorResponseDoesNotLeakBody(t *testing.T) {
	server := newErrorES(t)
	defer server.Close()

	t.Setenv("ELASTICSEARCH_URL", server.URL)
	t.Setenv("ELASTICSEARCH_USERNAME", "elastic")
	t.Setenv("ELASTICSEARCH_PASSWORD", "changeme")

	logBuf, restore := newDebugLogBuffer(t)
	defer restore()

	ctx := context.Background()
	client, err := NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{"term": map[string]interface{}{"customer_id": "JPMC-42"}},
	}
	_, gotErr := client.Search(ctx, "transactions", query)
	if gotErr == nil {
		t.Fatal("expected an error from Search on an error response, got nil")
	}

	errStr := gotErr.Error()
	if strings.Contains(errStr, sensitiveSentinel) {
		t.Fatalf("consumer-facing search error LEAKED the sensitive sentinel: %q", errStr)
	}
	// The consumer-facing error should carry the safe HTTP status classifier.
	if !strings.Contains(errStr, "400") {
		t.Errorf("expected the HTTP status in the consumer-facing error, got: %q", errStr)
	}

	// The raw response body (with the sentinel) MUST be present at DEBUG only.
	logs := logBuf.String()
	if !strings.Contains(logs, sensitiveSentinel) {
		t.Errorf("expected the raw error body (with sentinel) in DEBUG logs, not found in:\n%s", logs)
	}
	if !strings.Contains(logs, `"msg":"es search error response"`) {
		t.Errorf("expected the debug-level 'es search error response' log line, not found in:\n%s", logs)
	}
}
