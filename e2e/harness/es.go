//go:build e2e

package harness

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ESClient is a minimal Elasticsearch HTTPS client used by the harness for
// seeding and for issuing the direct "equivalent" _search calls. It talks to ES
// over TLS using the CA cert produced by the compose `setup` service.
type ESClient struct {
	BaseURL  string // e.g. https://localhost:9200
	Username string
	Password string
	http     *http.Client
}

// NewESClient builds a client that trusts the given CA cert file. If caCertPath
// is empty the client trusts the system pool only (not recommended for the
// self-signed local stack).
func NewESClient(baseURL, username, password, caCertPath string) (*ESClient, error) {
	tlsCfg := &tls.Config{}
	if caCertPath != "" {
		pem, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert %s: %w", caCertPath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("CA cert %s contained no valid certificates", caCertPath)
		}
		tlsCfg.RootCAs = pool
	}
	return &ESClient{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Username: username,
		Password: password,
		http: &http.Client{
			Timeout:   60 * time.Second,
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		},
	}, nil
}

// do issues an authenticated request and returns the status code + body.
func (c *ESClient) do(ctx context.Context, method, path, contentType string, body []byte) (int, []byte, error) {
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.SetBasicAuth(c.Username, c.Password)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, b, nil
}

// WaitReady polls the cluster root until it authenticates successfully or the
// context is cancelled.
func (c *ESClient) WaitReady(ctx context.Context) error {
	deadline := time.Now().Add(3 * time.Minute)
	var last string
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		code, body, err := c.do(ctx, http.MethodGet, "/", "", nil)
		if err == nil && code == http.StatusOK {
			return nil
		}
		if err != nil {
			last = err.Error()
		} else {
			last = fmt.Sprintf("status %d: %s", code, tail(string(body), 200))
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("elasticsearch not ready: %s", last)
}

// PutMapping creates an index with the given raw mapping JSON (the file content
// of indices/<index>.mapping.json). The file base name (without .mapping.json)
// is the index name.
func (c *ESClient) PutIndex(ctx context.Context, index string, mappingJSON []byte) error {
	code, body, err := c.do(ctx, http.MethodPut, "/"+index, "application/json", mappingJSON)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("PUT /%s => %d: %s", index, code, tail(string(body), 800))
	}
	return nil
}

// Bulk sends an NDJSON bulk payload and fails if any item errored.
func (c *ESClient) Bulk(ctx context.Context, ndjson []byte) error {
	// The _bulk API requires a trailing newline.
	if len(ndjson) == 0 || ndjson[len(ndjson)-1] != '\n' {
		ndjson = append(ndjson, '\n')
	}
	code, body, err := c.do(ctx, http.MethodPost, "/_bulk", "application/x-ndjson", ndjson)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("POST /_bulk => %d: %s", code, tail(string(body), 800))
	}
	var parsed struct {
		Errors bool                     `json:"errors"`
		Items  []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("parsing bulk response: %w", err)
	}
	if parsed.Errors {
		return fmt.Errorf("_bulk reported item errors: %s", tail(string(body), 1200))
	}
	return nil
}

// Refresh forces a refresh so freshly bulk-loaded docs are searchable.
func (c *ESClient) Refresh(ctx context.Context, index string) error {
	code, body, err := c.do(ctx, http.MethodPost, "/"+index+"/_refresh", "", nil)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("POST /%s/_refresh => %d: %s", index, code, tail(string(body), 400))
	}
	return nil
}

// GetMapping returns GET /<index>/_mapping decoded as a generic map.
func (c *ESClient) GetMapping(ctx context.Context, index string) (map[string]interface{}, error) {
	code, body, err := c.do(ctx, http.MethodGet, "/"+index+"/_mapping", "", nil)
	if err != nil {
		return nil, err
	}
	if code >= 300 {
		return nil, fmt.Errorf("GET /%s/_mapping => %d: %s", index, code, tail(string(body), 400))
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Search issues POST /<index>/_search with the given DSL body and returns the
// raw response body.
func (c *ESClient) Search(ctx context.Context, index string, dsl []byte) ([]byte, error) {
	code, body, err := c.do(ctx, http.MethodPost, "/"+index+"/_search", "application/json", dsl)
	if err != nil {
		return nil, err
	}
	if code >= 300 {
		return body, fmt.Errorf("POST /%s/_search => %d: %s", index, code, tail(string(body), 800))
	}
	return body, nil
}

// SetKibanaSystemPassword sets the kibana_system built-in user's password so
// the kibana container can authenticate. Only used for kibana-sample cases.
func (c *ESClient) SetKibanaSystemPassword(ctx context.Context, password string) error {
	payload, _ := json.Marshal(map[string]string{"password": password})
	code, body, err := c.do(ctx, http.MethodPost, "/_security/user/kibana_system/_password", "application/json", payload)
	if err != nil {
		return err
	}
	if code >= 300 {
		return fmt.Errorf("set kibana_system password => %d: %s", code, tail(string(body), 400))
	}
	return nil
}

// RawGet issues an arbitrary authenticated GET (used by init/*.http scripts and
// readiness checks).
func (c *ESClient) RawGet(ctx context.Context, path string) (int, []byte, error) {
	return c.do(ctx, http.MethodGet, path, "", nil)
}

// RawRequest issues an arbitrary authenticated request (used by init/*.http).
func (c *ESClient) RawRequest(ctx context.Context, method, path, contentType string, body []byte) (int, []byte, error) {
	return c.do(ctx, method, path, contentType, body)
}
