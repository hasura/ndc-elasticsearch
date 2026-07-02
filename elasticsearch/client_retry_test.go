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
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
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

// ---------------------------------------------------------------------------
// Helpers for concurrent tests
// ---------------------------------------------------------------------------

// reauthTrackingServer builds a fake ES server where:
//   - HEAD / succeeds always (Ping).
//   - GET / returns a minimal version doc (client init).
//   - All other paths (_search) return HTTP 401 until reauthDone is set, then
//     HTTP 200. reauthDone is set atomically the moment the *first* HEAD /
//     arrives after setup is marked complete, so it flips exactly once — during
//     the single reauthentication that should happen under the fix.
//
// pingCount counts HEAD / requests received after setupDone is set; it must
// equal 1 for the test to pass (exactly one reauthentication).
// bodies collects the raw body of every _search request received.
func reauthTrackingServer(
	t *testing.T,
	setupDone *atomic.Bool,
	reauthDone *atomic.Bool,
	pingCount *atomic.Int32,
	mu *sync.Mutex,
	bodies *[]string,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/":
			if setupDone.Load() {
				pingCount.Add(1)
				reauthDone.Store(true)
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":{"number":"8.0.0"}}`))
		default:
			b, _ := io.ReadAll(r.Body)
			if mu != nil {
				mu.Lock()
				*bodies = append(*bodies, string(b))
				mu.Unlock()
			}
			if !reauthDone.Load() {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"took":1,"hits":{"total":{"value":0},"hits":[]}}`))
		}
	}))
}

// ---------------------------------------------------------------------------
// TestConcurrentSearchSingleReauth — the thundering-herd test
// ---------------------------------------------------------------------------

// TestConcurrentSearchSingleReauth fires N goroutines that all receive HTTP 401
// on their first _search attempt (token expired). Under the fix, reauthMu
// ensures only ONE goroutine calls Reauthenticate; the others wait, detect that
// e.client has already been replaced, and skip straight to their retry.
//
// Assertions:
//  1. Every goroutine's Search() call succeeds (no errors).
//  2. Exactly one reauthentication (one POST-setup Ping) occurred.
//
// Without the fix (no reauthMu), all N goroutines would race to write
// e.client simultaneously, triggering the race detector and potentially causing
// redundant — or conflicting — credential refreshes.
//
//	go test -v -race -run TestConcurrentSearchSingleReauth ./elasticsearch/
func TestConcurrentSearchSingleReauth(t *testing.T) {
	const N = 20

	var (
		setupDone  atomic.Bool
		reauthDone atomic.Bool
		pingCount  atomic.Int32
		mu         sync.Mutex
		bodies     []string
	)

	server := reauthTrackingServer(t, &setupDone, &reauthDone, &pingCount, &mu, &bodies)
	defer server.Close()

	t.Setenv("ELASTICSEARCH_URL", server.URL)
	t.Setenv("ELASTICSEARCH_USERNAME", "elastic")
	t.Setenv("ELASTICSEARCH_PASSWORD", "changeme")

	ctx := context.Background()
	client, err := NewClient(ctx)
	require.NoError(t, err)
	setupDone.Store(true)

	errs := make([]error, N)
	var wg sync.WaitGroup
	for i := range N {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			q := map[string]interface{}{
				"query": map[string]interface{}{
					"term": map[string]interface{}{"goroutine": i},
				},
			}
			_, errs[i] = client.Search(ctx, "idx", q)
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		require.NoError(t, e, "goroutine %d returned an error", i)
	}

	got := int(pingCount.Load())
	if got != 1 {
		t.Errorf("expected exactly 1 reauthentication, got %d — thundering herd not prevented", got)
	}
}

// ---------------------------------------------------------------------------
// TestConcurrentSearchBodiesPreserved
// ---------------------------------------------------------------------------

// TestConcurrentSearchBodiesPreserved verifies that each goroutine's retry
// carries its own, non-empty query body — not an empty payload and not another
// goroutine's query. It uses the same reauthDone flip mechanism so all first
// attempts get 401 and all retries get 200.
//
//	go test -v -race -run TestConcurrentSearchBodiesPreserved ./elasticsearch/
func TestConcurrentSearchBodiesPreserved(t *testing.T) {
	const N = 10

	var (
		setupDone  atomic.Bool
		reauthDone atomic.Bool
		pingCount  atomic.Int32
		mu         sync.Mutex
		bodies     []string
	)

	server := reauthTrackingServer(t, &setupDone, &reauthDone, &pingCount, &mu, &bodies)
	defer server.Close()

	t.Setenv("ELASTICSEARCH_URL", server.URL)
	t.Setenv("ELASTICSEARCH_USERNAME", "elastic")
	t.Setenv("ELASTICSEARCH_PASSWORD", "changeme")

	ctx := context.Background()
	client, err := NewClient(ctx)
	require.NoError(t, err)
	setupDone.Store(true)

	// Build N queries, each uniquely identified by goroutine_id.
	queries := make([]map[string]interface{}, N)
	for i := range queries {
		queries[i] = map[string]interface{}{
			"query": map[string]interface{}{
				"term": map[string]interface{}{"goroutine_id": i},
			},
		}
	}

	errs := make([]error, N)
	var wg sync.WaitGroup
	for i := range N {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = client.Search(ctx, "idx", queries[i])
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		require.NoError(t, e, "goroutine %d returned an error", i)
	}

	mu.Lock()
	got := append([]string(nil), bodies...)
	mu.Unlock()

	// No attempt — first or retry — may ever send an empty body.
	for i, b := range got {
		if len(trimWS(b)) == 0 {
			t.Errorf("attempt %d sent an empty body; ES would treat this as match_all", i)
		}
	}

	// Every goroutine's query must appear at least once among the received
	// bodies. If a retry dropped the body the query would be absent.
	for i, q := range queries {
		encoded, _ := json.Marshal(q)
		found := false
		for _, b := range got {
			if jsonEqual(t, b, string(encoded)) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("goroutine %d query not found in any received body; query was dropped", i)
		}
	}
}

// ---------------------------------------------------------------------------
// TestSearchTransportError
// ---------------------------------------------------------------------------

// TestSearchTransportError verifies that a transport-level failure on the first
// attempt (the TCP connection is closed abruptly, so req.Do returns err != nil
// and res == nil) is surfaced as an error rather than causing a nil-pointer
// panic on res.IsError().
//
//	go test -v -run TestSearchTransportError ./elasticsearch/
func TestSearchTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":{"number":"8.0.0"}}`))
		default:
			// Abruptly close the connection to produce a transport error.
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijacking not supported", http.StatusInternalServerError)
				return
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				return
			}
			conn.Close()
		}
	}))
	defer server.Close()

	t.Setenv("ELASTICSEARCH_URL", server.URL)
	t.Setenv("ELASTICSEARCH_USERNAME", "elastic")
	t.Setenv("ELASTICSEARCH_PASSWORD", "changeme")

	ctx := context.Background()
	client, err := NewClient(ctx)
	require.NoError(t, err)

	_, err = client.Search(ctx, "idx", map[string]interface{}{
		"query": map[string]interface{}{"match_all": map[string]interface{}{}},
	})
	if err == nil {
		t.Fatal("expected an error on transport failure, got nil")
	}
	t.Logf("got expected transport error: %v", err)
}

// ---------------------------------------------------------------------------
// TestSearchReauthFails
// ---------------------------------------------------------------------------

// TestSearchReauthFails verifies that when a _search returns 401 and the
// subsequent Reauthenticate call itself fails (the Ping during Authenticate
// also returns a non-2xx), Search surfaces an error rather than retrying with
// a stale or nil client.
//
//	go test -v -run TestSearchReauthFails ./elasticsearch/
func TestSearchReauthFails(t *testing.T) {
	// Allow the initial NewClient ping to succeed (count == 1), then fail all
	// subsequent pings so that Reauthenticate cannot complete.
	var pingCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/":
			n := int(pingCount.Add(1))
			if n == 1 {
				// Initial setup ping — succeed.
				w.WriteHeader(http.StatusOK)
				return
			}
			// All subsequent pings (reauth attempts) — fail.
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":{"number":"8.0.0"}}`))
		default:
			// Every _search returns 401 to trigger reauthentication.
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		}
	}))
	defer server.Close()

	t.Setenv("ELASTICSEARCH_URL", server.URL)
	t.Setenv("ELASTICSEARCH_USERNAME", "elastic")
	t.Setenv("ELASTICSEARCH_PASSWORD", "changeme")

	ctx := context.Background()
	client, err := NewClient(ctx)
	require.NoError(t, err)

	_, err = client.Search(ctx, "idx", map[string]interface{}{
		"query": map[string]interface{}{"term": map[string]interface{}{"id": "test"}},
	})
	if err == nil {
		t.Fatal("expected Search to return an error when Reauthenticate fails, got nil")
	}
	t.Logf("got expected error: %v", err)
}

// ---------------------------------------------------------------------------
// TestConcurrentSearchNoReauthIfClientAlreadyReplaced
// ---------------------------------------------------------------------------

// TestConcurrentSearchNoReauthIfClientAlreadyReplaced is the unit-level proof
// of the "skip reauth if already replaced" logic. It checks that a goroutine
// which arrives at reauthMu *after* another goroutine has already rotated the
// client does NOT call Reauthenticate a second time.
//
// It does this by running two sequential waves of the same scenario:
//   - Wave 1: first goroutine triggers a real reauth; second goroutine must
//     detect the new client and skip.
//   - Both goroutines must get successful responses.
//
// The ping-count assertion is inherited from TestConcurrentSearchSingleReauth;
// here we concentrate on correctness for the two-goroutine case so failures
// are easier to diagnose.
//
//	go test -v -race -run TestConcurrentSearchNoReauthIfClientAlreadyReplaced ./elasticsearch/
func TestConcurrentSearchNoReauthIfClientAlreadyReplaced(t *testing.T) {
	var (
		setupDone  atomic.Bool
		reauthDone atomic.Bool
		pingCount  atomic.Int32
	)

	server := reauthTrackingServer(t, &setupDone, &reauthDone, &pingCount, nil, nil)
	defer server.Close()

	t.Setenv("ELASTICSEARCH_URL", server.URL)
	t.Setenv("ELASTICSEARCH_USERNAME", "elastic")
	t.Setenv("ELASTICSEARCH_PASSWORD", "changeme")

	ctx := context.Background()
	client, err := NewClient(ctx)
	require.NoError(t, err)
	setupDone.Store(true)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = client.Search(ctx, "idx", map[string]interface{}{
				"query": map[string]interface{}{"term": map[string]interface{}{"i": i}},
			})
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		require.NoError(t, e, "goroutine %d", i)
	}
	if got := int(pingCount.Load()); got != 1 {
		t.Errorf("expected 1 reauthentication for 2 concurrent 401s, got %d", got)
	}
}
