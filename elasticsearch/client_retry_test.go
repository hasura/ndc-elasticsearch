package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// newFakeES returns an httptest server that emulates an Elasticsearch cluster
// which fails the first _search with 401 (token expired mid-traffic) and then
// succeeds. It records the body received on every _search attempt into bodies.
func newFakeES(t *testing.T, bodies *[]string, mu *sync.Mutex) *httptest.Server {
	t.Helper()
	var attempts int
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":{"number":"8.0.0"}}`))
		default:
			bodyBytes, _ := io.ReadAll(r.Body)
			mu.Lock()
			attempts++
			attempt := attempts
			*bodies = append(*bodies, string(bodyBytes))
			mu.Unlock()
			if attempt == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"took":1,"hits":{"total":{"value":0},"hits":[]}}`))
		}
	}))
}

// TestSearchRetryBodyOn401 is a hermetic functional test for the "dropped
// payload on intermittent retry" bug (JPMorgan Chase support ticket #15000).
//
// It stands up a fake Elasticsearch server that returns 401 on the first
// _search call (simulating a token expiring mid-traffic) and 200 afterwards.
// It captures the request body received on EVERY _search attempt and asserts
// that the retry, sent after re-authentication, carries the SAME, non-empty
// query body as the first attempt.
//
//	go test -v -run TestSearchRetryBodyOn401 ./elasticsearch/
//
// To manually reproduce the OLD buggy behaviour (retry sends an empty body),
// comment out the body-rebuild line in search() in client.go:
//
//	// req.Body = bytes.NewReader(body)   // <- comment this out before the retry req.Do
//
// and re-run this test: it will fail with "BUG REPRODUCED", because the retry
// then reuses the already-drained reader and sends an empty body.
func TestSearchRetryBodyOn401(t *testing.T) {
	var (
		mu           sync.Mutex
		searchBodies []string
	)

	server := newFakeES(t, &searchBodies, &mu)
	defer server.Close()

	// Point the connector at the fake server. These are required by
	// getConfigFromEnv so that Reauthenticate succeeds against the fake server.
	t.Setenv("ELASTICSEARCH_URL", server.URL)
	t.Setenv("ELASTICSEARCH_USERNAME", "elastic")
	t.Setenv("ELASTICSEARCH_PASSWORD", "changeme")

	ctx := context.Background()
	client, err := NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"customer_id": "JPMC-42"},
		},
		"size": 10,
	}
	expectedBody, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("failed to marshal query: %v", err)
	}

	if _, err := client.Search(ctx, "transactions", query); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(searchBodies) != 2 {
		t.Fatalf("expected exactly 2 _search attempts (initial + retry), got %d", len(searchBodies))
	}

	// The first attempt must always carry the full query body.
	if !jsonEqual(t, searchBodies[0], string(expectedBody)) {
		t.Fatalf("first attempt body mismatch:\n  got:  %q\n  want: %q", searchBodies[0], expectedBody)
	}

	// The crux of the bug: the retry must carry the SAME, non-empty body.
	retryBody := searchBodies[1]
	if len(trimWS(retryBody)) == 0 {
		t.Fatalf("BUG REPRODUCED: retry sent an EMPTY body (Elasticsearch would treat this as match_all and return unfiltered results). retry body=%q", retryBody)
	}
	if !jsonEqual(t, retryBody, string(expectedBody)) {
		t.Fatalf("retry body mismatch:\n  got:  %q\n  want: %q", retryBody, expectedBody)
	}

	t.Logf("OK: both attempts sent identical, non-empty query body: %s", expectedBody)
}

// TestSearchPerAttemptLogging verifies the per-attempt request logging: the
// connector logs the actual _search body and target index it sends on EVERY
// attempt, and the retry log line carries the literal "Retry Query" marker so
// retries are easy to grep. The logged body reflects what is ACTUALLY sent, so
// the "Retry Query" line carries the same non-empty body as the first attempt.
//
//	go test -v -run TestSearchPerAttemptLogging ./elasticsearch/
func TestSearchPerAttemptLogging(t *testing.T) {
	var (
		mu           sync.Mutex
		searchBodies []string
	)
	server := newFakeES(t, &searchBodies, &mu)
	defer server.Close()

	t.Setenv("ELASTICSEARCH_URL", server.URL)
	t.Setenv("ELASTICSEARCH_USERNAME", "elastic")
	t.Setenv("ELASTICSEARCH_PASSWORD", "changeme")

	// Capture logs: GetLogger falls back to slog.Default() when no logger is set
	// on the context, so redirect the default logger to a debug-level buffer.
	var logBuf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	ctx := context.Background()
	client, err := NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{"term": map[string]interface{}{"customer_id": "JPMC-42"}},
	}
	if _, err := client.Search(ctx, "transactions", query); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	logs := logBuf.String()
	t.Logf("captured logs:\n%s", logs)

	// Per-attempt logging: first attempt logged as "Query".
	if !strings.Contains(logs, `"msg":"Query"`) {
		t.Errorf(`expected a per-attempt "Query" log line, not found in:\n%s`, logs)
	}
	// Retry attempt logged with the literal "Retry Query" marker.
	if !strings.Contains(logs, `"msg":"Retry Query"`) {
		t.Errorf(`expected a "Retry Query" marked log line, not found in:\n%s`, logs)
	}
	// The log must carry the target index.
	if !strings.Contains(logs, `"index":"transactions"`) {
		t.Errorf(`expected the target index in the log line, not found in:\n%s`, logs)
	}

	// body_bytes must be non-zero on both attempts, proving the retry carries a
	// real payload (body_bytes=0 would mean the bug is present).
	if logFieldForMsg(t, logs, "Query", "body_bytes") == "0" || logFieldForMsg(t, logs, "Query", "body_bytes") == "" {
		t.Errorf("expected non-zero body_bytes on first attempt")
	}
	retryBodyBytes := logFieldForMsg(t, logs, "Retry Query", "body_bytes")
	if retryBodyBytes == "0" || retryBodyBytes == "" {
		t.Fatalf("expected non-zero body_bytes on retry attempt, got %q", retryBodyBytes)
	}
	t.Logf("retry logged body_bytes=%s", retryBodyBytes)
}

// logFieldForMsg returns the value of field for the first JSON log record whose
// "msg" equals msg.
func logFieldForMsg(t *testing.T, logs, msg, field string) string {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec["msg"] == msg {
			switch v := rec[field].(type) {
			case string:
				return v
			case float64:
				return fmt.Sprintf("%g", v)
			}
		}
	}
	return ""
}

// jsonEqual compares two JSON documents for semantic equality (ignoring key
// order and insignificant whitespace).
func jsonEqual(t *testing.T, a, b string) bool {
	t.Helper()
	var av, bv interface{}
	if err := json.Unmarshal([]byte(a), &av); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &bv); err != nil {
		return false
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return string(ab) == string(bb)
}

func trimWS(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r != ' ' && r != '\n' && r != '\t' && r != '\r' {
			out = append(out, r)
		}
	}
	return string(out)
}
