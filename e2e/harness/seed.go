//go:build e2e

package harness

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Seed applies a case's seed inputs into a fresh Elasticsearch, in this order:
//
//  1. indices/*.mapping.json  -> PUT /<index>          (schema first)
//  2. init/*.sh | init/*.http -> arbitrary ES setup    (aliases, ingest
//     pipelines, settings — created BEFORE data so they apply at ingest time)
//  3. data/*.ndjson           -> POST /_bulk + refresh
//  4. case.yaml kibana_sample -> load an elastic.co sample dataset via Kibana
//
// (This ordering runs init before data on purpose so ingest pipelines/aliases
// declared by a case are in place when documents are indexed; see e2e/README.md.)
func Seed(ctx context.Context, s *Stack, es *ESClient, c Case) error {
	// 1. index mappings
	for _, path := range c.IndexMappings {
		index := indexNameFromMappingFile(path)
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := es.PutIndex(ctx, index, body); err != nil {
			return fmt.Errorf("creating index %q from %s: %w", index, filepath.Base(path), err)
		}
	}

	// 2. init scripts / http files
	for _, path := range c.InitScripts {
		if err := runInitScript(ctx, s, es, path); err != nil {
			return fmt.Errorf("init %s: %w", filepath.Base(path), err)
		}
	}

	// 3. bulk data + refresh
	touched := map[string]bool{}
	for _, path := range c.DataFiles {
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := es.Bulk(ctx, body); err != nil {
			return fmt.Errorf("bulk-loading %s: %w", filepath.Base(path), err)
		}
		for _, idx := range indicesInBulk(body) {
			touched[idx] = true
		}
	}
	for idx := range touched {
		if err := es.Refresh(ctx, idx); err != nil {
			return err
		}
	}

	// 4. kibana sample data
	if c.Meta.KibanaSample != "" {
		if err := s.StartKibana(ctx, es); err != nil {
			return err
		}
		if err := loadKibanaSample(ctx, s.KibanaBaseURL(), c.Meta.KibanaSample); err != nil {
			return err
		}
		// The sample data lands in a data stream; make sure it's searchable.
		_ = es.Refresh(ctx, "kibana_sample_data_"+c.Meta.KibanaSample)
	}

	return nil
}

// indexNameFromMappingFile turns "indices/products.mapping.json" -> "products".
func indexNameFromMappingFile(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".mapping.json")
}

// indicesInBulk scans an NDJSON bulk body for the "_index" of each action line
// so we know which indices to refresh.
func indicesInBulk(body []byte) []string {
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	expectAction := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if expectAction {
			// action line: {"index":{"_index":"foo",...}}
			if idx := extractIndexField(line); idx != "" {
				seen[idx] = true
			}
			expectAction = false
		} else {
			expectAction = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

// extractIndexField pulls the "_index" value from a bulk action line without a
// full JSON parse dependency on structure.
func extractIndexField(line string) string {
	const key = `"_index"`
	i := strings.Index(line, key)
	if i < 0 {
		return ""
	}
	rest := line[i+len(key):]
	c := strings.IndexByte(rest, ':')
	if c < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[c+1:])
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// runInitScript executes an init file. `.sh` files are run with the case's ES
// connection details exported as env vars (ES_URL, ES_USER, ES_PASS, ES_CACERT,
// ES_INDEX-agnostic). `.http` files are a tiny request DSL (see README).
func runInitScript(ctx context.Context, s *Stack, es *ESClient, path string) error {
	switch {
	case strings.HasSuffix(path, ".sh"):
		env := []string{
			"ES_URL=" + es.BaseURL,
			"ES_USER=" + es.Username,
			"ES_PASS=" + es.Password,
			"ES_CACERT=" + s.CACert,
		}
		_, err := mustRun(ctx, filepath.Dir(path), env, "bash", path)
		return err
	case strings.HasSuffix(path, ".http"):
		return runHTTPFile(ctx, es, path)
	default:
		return fmt.Errorf("unsupported init file type: %s", filepath.Base(path))
	}
}

// runHTTPFile executes a minimal HTTP request DSL against ES. Each request is:
//
//	METHOD /path
//	<optional JSON body until a blank line or EOF or the next METHOD line>
//
// Lines starting with '#' are comments. Requests are separated by blank lines.
// Example:
//
//	PUT /_ingest/pipeline/my-pipeline
//	{ "processors": [ ... ] }
//
//	POST /_aliases
//	{ "actions": [ ... ] }
func runHTTPFile(ctx context.Context, es *ESClient, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")

	var method, reqPath string
	var body strings.Builder
	flush := func() error {
		if method == "" {
			return nil
		}
		code, respBody, err := es.RawRequest(ctx, method, reqPath, "application/json",
			[]byte(strings.TrimSpace(body.String())))
		if err != nil {
			return err
		}
		if code >= 300 {
			return fmt.Errorf("%s %s => %d: %s", method, reqPath, code, tail(string(respBody), 400))
		}
		method, reqPath = "", ""
		body.Reset()
		return nil
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isRequestLine(trimmed) {
			if err := flush(); err != nil {
				return err
			}
			parts := strings.Fields(trimmed)
			method, reqPath = parts[0], parts[1]
			continue
		}
		if trimmed == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	return flush()
}

func isRequestLine(line string) bool {
	for _, m := range []string{"GET ", "PUT ", "POST ", "DELETE ", "HEAD "} {
		if strings.HasPrefix(line, m) {
			return true
		}
	}
	return false
}

// loadKibanaSample POSTs to the Kibana sample-data API. dataset is one of
// logs|ecommerce|flights.
func loadKibanaSample(ctx context.Context, kibanaURL, dataset string) error {
	url := fmt.Sprintf("%s/api/sample_data/%s", strings.TrimRight(kibanaURL, "/"), dataset)
	client := &http.Client{Timeout: 3 * time.Minute}

	// Kibana can take a moment after "healthy" before the sample_data API is
	// ready; retry a few times.
	deadline := time.Now().Add(3 * time.Minute)
	var last string
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("kbn-xsrf", "true")
		req.SetBasicAuth("elastic", elasticPassword)
		resp, err := client.Do(req)
		if err != nil {
			last = err.Error()
			time.Sleep(3 * time.Second)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		last = "status " + resp.Status
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("loading kibana sample %q: %s", dataset, last)
}
