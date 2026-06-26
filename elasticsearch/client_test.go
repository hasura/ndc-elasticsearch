package elasticsearch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// testModeEnvVar toggles which search implementation the functional test below
// exercises, so the same assertion can be run against both the buggy and the
// fixed code without editing any source:
//
//	ES_401_TEST_MODE=buggy go test -run TestSearch401RetrySendsSameBody ./elasticsearch/  -> FAILS (reproduces the bug)
//	ES_401_TEST_MODE=fixed go test -run TestSearch401RetrySendsSameBody ./elasticsearch/  -> PASSES (verifies the fix)
//
// "fixed" is the default when the variable is unset.
const testModeEnvVar = "ES_401_TEST_MODE"

// recordingTransport is a fake http.RoundTripper standing in for Elasticsearch.
// It returns 401 on the first attempt (to trigger the connector's re-auth +
// retry path) and 200 on every subsequent attempt, while recording the exact
// request body it received on each attempt. The recorded retry body is the
// crux of the bug: a correct connector must replay the original query body.
type recordingTransport struct {
	bodies [][]byte
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}
	rt.bodies = append(rt.bodies, body)

	header := http.Header{
		"Content-Type": []string{"application/json"},
		// Required so the go-elasticsearch product check passes on 2xx responses.
		"X-Elastic-Product": []string{"Elasticsearch"},
	}

	// First attempt -> 401 Unauthorized, to drive the re-auth + retry branch.
	if len(rt.bodies) == 1 {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Status:     "401 Unauthorized",
			Header:     header,
			Body:       io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)),
			Request:    req,
		}, nil
	}

	// Retry -> 200 OK with a minimal valid search response.
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(`{"took":1,"timed_out":false,"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`)),
		Request:    req,
	}, nil
}

// buggySearch is a faithful copy of the pre-fix search() body handling
// introduced in commit 3431d53 (PR #72, first shipped in v1.5.2). It reuses the
// already-drained request body on the 401 retry, so the retried _search carries
// an empty body. It exists only to let this test reproduce the original bug via
// ES_401_TEST_MODE=buggy without checking out an old revision.
func buggySearch(e *Client, ctx context.Context, o ...func(*esapi.SearchRequest)) (*esapi.Response, error) {
	req := &esapi.SearchRequest{}
	for _, opt := range o {
		opt(req)
	}

	res, err := req.Do(ctx, e.client)
	if res.IsError() {
		if res.StatusCode == 401 {
			if err = e.reauth(ctx); err != nil {
				return nil, fmt.Errorf("error: %s", err)
			}
			res, err = req.Do(ctx, e.client) // reuses the drained body -> empty payload
			if err != nil {
				return nil, fmt.Errorf("error: %s", err)
			}
		} else {
			return nil, fmt.Errorf("error while querying: %s", res.String())
		}
	}
	return res, err
}

// TestSearch401RetrySendsSameBody is a functional test for the 401-retry
// payload-drop bug. It drives a real *Client through a 401 -> re-auth -> retry
// cycle against a fake Elasticsearch transport and asserts that the body sent on
// the retry is byte-for-byte identical to the body sent on the first attempt.
//
// Run it both ways (the assertion is identical; only the implementation differs):
//
//	ES_401_TEST_MODE=buggy go test -v -run TestSearch401RetrySendsSameBody ./elasticsearch/   # reproduces the bug -> FAIL
//	ES_401_TEST_MODE=fixed go test -v -run TestSearch401RetrySendsSameBody ./elasticsearch/   # verifies the fix   -> PASS
func TestSearch401RetrySendsSameBody(t *testing.T) {
	mode := os.Getenv(testModeEnvVar)
	if mode == "" {
		mode = "fixed"
	}

	ctx := context.Background()
	queryBody := []byte(`{"query":{"term":{"title":"hasura"}}}`)

	transport := &recordingTransport{}
	esClient, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses:    []string{"http://elasticsearch.invalid:9200"},
		Transport:    transport,
		DisableRetry: true, // keep attempt counting deterministic; 401 isn't retried by the transport anyway
	})
	if err != nil {
		t.Fatalf("failed to build elasticsearch client: %v", err)
	}

	e := &Client{
		client: esClient,
		// Keep the fake transport in place on re-auth (the real Reauthenticate
		// would rebuild the client and drop our transport).
		reauthenticate: func(context.Context) error { return nil },
	}

	opts := []func(*esapi.SearchRequest){
		func(r *esapi.SearchRequest) { r.Index = []string{"test-index"} },
		// Mirror production: the body is a *bytes.Buffer, exactly as Search() builds it.
		func(r *esapi.SearchRequest) { r.Body = bytes.NewBuffer(queryBody) },
	}

	t.Logf("running in %q mode (set %s=buggy or =fixed to toggle)", mode, testModeEnvVar)
	switch mode {
	case "buggy":
		_, err = buggySearch(e, ctx, opts...)
	case "fixed":
		_, err = e.search(ctx, opts...)
	default:
		t.Fatalf("unknown %s=%q (want \"buggy\" or \"fixed\")", testModeEnvVar, mode)
	}
	if err != nil {
		t.Fatalf("search returned an unexpected error: %v", err)
	}

	if len(transport.bodies) != 2 {
		t.Fatalf("expected exactly 2 requests to ES (initial + retry), got %d", len(transport.bodies))
	}

	firstBody := transport.bodies[0]
	retryBody := transport.bodies[1]
	t.Logf("first-attempt body: %s", firstBody)
	t.Logf("retry body:         %q (len=%d)", retryBody, len(retryBody))

	// Sanity: the first attempt must have carried the real query.
	if strings.TrimSpace(string(firstBody)) != string(queryBody) {
		t.Fatalf("first attempt sent an unexpected body: got %q, want %q", firstBody, queryBody)
	}

	// The crux: the 401 retry must transmit the SAME query body. The buggy
	// implementation sends an empty body here (Elasticsearch would treat it as
	// match_all), so this assertion fails in ES_401_TEST_MODE=buggy and passes
	// once the fix re-seeds the request body before retrying.
	if strings.TrimSpace(string(retryBody)) != string(queryBody) {
		t.Errorf("401 retry sent the wrong body (payload dropped):\n  got:  %q (len=%d)\n  want: %q\n"+
			"An empty/short body means the retry was sent without the query, which Elasticsearch "+
			"interprets as match_all. This is the bug from commit 3431d53 / PR #72.",
			retryBody, len(retryBody), queryBody)
	}
}

// TestSearch401RetryLogsBodyPerAttempt verifies the per-attempt request logging
// requested in support ticket #15000: the connector must log the actual _search
// request (target index + body) it sends on EACH attempt, and the post-401
// retry log line must carry the literal marker "Retry Query" so it can be
// grepped apart from first-attempt "Query" lines.
//
// It captures the connector's structured logs (slog) and asserts both markers
// are emitted with the real query body. Because the fix re-seeds the request
// body before every attempt, the logged body matches what is actually sent over
// the wire on both the first attempt and the retry.
func TestSearch401RetryLogsBodyPerAttempt(t *testing.T) {
	// Route connector.GetLogger's fallback (no logger in ctx) at debug level into
	// a buffer we can assert on. slog.Default is global, so restore it after.
	var logBuf bytes.Buffer
	prevDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prevDefault)

	ctx := context.Background()
	queryBody := []byte(`{"query":{"term":{"title":"hasura"}}}`)

	transport := &recordingTransport{}
	esClient, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses:    []string{"http://elasticsearch.invalid:9200"},
		Transport:    transport,
		DisableRetry: true,
	})
	if err != nil {
		t.Fatalf("failed to build elasticsearch client: %v", err)
	}

	e := &Client{
		client:         esClient,
		reauthenticate: func(context.Context) error { return nil },
	}

	opts := []func(*esapi.SearchRequest){
		func(r *esapi.SearchRequest) { r.Index = []string{"test-index"} },
		func(r *esapi.SearchRequest) { r.Body = bytes.NewBuffer(queryBody) },
	}

	if _, err = e.search(ctx, opts...); err != nil {
		t.Fatalf("search returned an unexpected error: %v", err)
	}

	logs := logBuf.String()
	t.Logf("captured logs:\n%s", logs)

	// First-attempt log line.
	if !strings.Contains(logs, `"msg":"Query"`) {
		t.Errorf("expected a first-attempt %q log line; got:\n%s", "Query", logs)
	}
	// Retry log line carrying the required marker.
	if !strings.Contains(logs, `"msg":"Retry Query"`) {
		t.Errorf("expected a %q marked log line on the 401 retry; got:\n%s", "Retry Query", logs)
	}
	// Both log lines must include the actual query body and target index. The
	// body is JSON-escaped inside the JSON log record, so assert on a stable
	// fragment of the query.
	if strings.Count(logs, `term`) < 2 || strings.Count(logs, `title`) < 2 {
		t.Errorf("expected the real query body to be logged on both attempts; got:\n%s", logs)
	}
	if strings.Count(logs, `"index":"test-index"`) < 2 {
		t.Errorf("expected the target index to be logged on both attempts; got:\n%s", logs)
	}
	// Sanity: the fake never sends credentials, but guard against accidentally
	// logging common auth fields from this path.
	for _, secret := range []string{"Authorization", "password", "apiKey", "api_key", "ServiceToken"} {
		if strings.Contains(logs, secret) {
			t.Errorf("log output unexpectedly contains a credential-like field %q:\n%s", secret, logs)
		}
	}
}
