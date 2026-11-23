//go:build e2e

package relationships

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hasura/ndc-elasticsearch/e2e/harness"
)

func TestMissingObjectWithCrossIndexRelationship(t *testing.T) {
	if os.Getenv("E2E") != "1" {
		t.Skip("run with E2E=1 and -tags e2e")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	env, err := harness.LoadEnv()
	check(t, "load e2e config", err)
	fixture := filepath.Join(env.E2EDir, "relationships", "testdata")
	c := harness.Case{
		Name: "missing_object_relationship",
		IndexMappings: []string{
			filepath.Join(fixture, "repro_source.mapping.json"),
			filepath.Join(fixture, "repro_target.mapping.json"),
		},
		DataFiles: []string{filepath.Join(fixture, "documents.ndjson")},
	}
	stack, err := harness.NewStack(env, c)
	check(t, "create stack", err)
	t.Cleanup(func() { stack.Down(context.Background()) })

	es, err := stack.StartElasticsearch(ctx)
	check(t, "start Elasticsearch", err)
	check(t, "seed both indices", harness.Seed(ctx, stack, es, c))
	t.Run("elasticsearch_omits_y", func(t *testing.T) {
		body, err := es.Search(ctx, "repro_source", []byte(`{"_source":["x","y.value","z"],"query":{"match_all":{}}}`))
		check(t, "search Elasticsearch", err)
		var result struct {
			Hits struct {
				Hits []struct {
					Source row `json:"_source"`
				} `json:"hits"`
			} `json:"hits"`
		}
		check(t, "decode Elasticsearch search", json.Unmarshal(body, &result))
		var rows []row
		for _, hit := range result.Hits.Hits {
			rows = append(rows, hit.Source)
		}
		assertRows(t, rows, `[{"x":"present","y":{"value":"populated"},"z":"present-z"},{"x":"missing","z":"missing-z"}]`)
	})

	check(t, "introspect connector", stack.Introspect(ctx))
	check(t, "start connector", stack.StartConnector(ctx))
	connectorURL := fmt.Sprintf("http://localhost:%d", stack.Ports["connector"])
	schema := ndcJSON(t, ctx, connectorURL+"/schema", "")
	caps := ndcJSON(t, ctx, connectorURL+"/capabilities", "")

	// Characterize the mismatch as it exists today; do not treat the null
	// under a non-nullable leaf as the intended contract for a future fix.
	const projected = `[{"x":"present","y":[{"value":"populated"}],"z":"present-z"},{"x":"missing","y":[{"value":null}],"z":"missing-z"}]`
	t.Run("ndc_non_null_leaf_and_null_value", func(t *testing.T) {
		var declared struct {
			Objects map[string]struct {
				Fields map[string]struct {
					Type map[string]any `json:"type"`
				} `json:"fields"`
			} `json:"object_types"`
		}
		check(t, "decode NDC schema", json.Unmarshal(schema, &declared))
		if got := declared.Objects["repro_source.y"].Fields["value"].Type; !reflect.DeepEqual(got, map[string]any{"type": "named", "name": "keyword"}) {
			t.Fatalf("expected a non-nullable keyword leaf, got %v", got)
		}
		body := ndcJSON(t, ctx, connectorURL+"/query", read(t, filepath.Join(fixture, "projection.json")))
		var sets []struct {
			Rows []row `json:"rows"`
		}
		check(t, "decode NDC query response", json.Unmarshal(body, &sets))
		if len(sets) != 1 {
			t.Fatalf("want one NDC rowset, got %s", body)
		}
		assertRows(t, sets[0].Rows, projected)
	})

	stageEngineMetadata(t, stack.Workdir, fixture, schema, caps)
	check(t, "start DDN engine", stack.StartEngine(ctx))
	for _, tc := range []struct{ name, query, want string }{
		{"projection_without_join", `{ sources { x y { value } z } }`, projected},
		{"join_without_y", `{ sources { x z target { label } } }`, `[{"x":"present","z":"present-z","target":{"label":"present-target"}},{"x":"missing","z":"missing-z","target":{"label":"missing-target"}}]`},
		{"join_with_y", `{ sources { x y { value } z target { label } } }`, `[{"x":"present","y":[{"value":"populated"}],"z":"present-z","target":{"label":"present-target"}},{"x":"missing","y":[{"value":null}],"z":"missing-z","target":{"label":"missing-target"}}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := harness.RunGraphQL(ctx, stack.EngineGraphQLURL(), tc.query, nil)
			check(t, "query DDN GraphQL", err)
			var data struct {
				Sources []row `json:"sources"`
			}
			check(t, "decode DDN rows", json.Unmarshal(res.Data, &data))
			assertRows(t, data.Sources, tc.want)
		})
	}
}

type row map[string]any

func assertRows(t *testing.T, actual []row, expected string) {
	t.Helper()
	var want []row
	check(t, "decode expected rows", json.Unmarshal([]byte(expected), &want))
	byKey := make(map[string]row, len(want))
	for _, r := range want {
		key, ok := r["x"].(string)
		if !ok {
			t.Fatalf("expected fixture row to have a string x: %#v", r)
		}
		byKey[key] = r
	}
	for _, got := range actual {
		key, _ := got["x"].(string)
		if !reflect.DeepEqual(got, byKey[key]) {
			t.Errorf("row %q: got %#v, want %#v", key, got, byKey[key])
		}
		delete(byKey, key)
	}
	if len(byKey) > 0 {
		t.Errorf("missing rows: %#v", byKey)
	}
}

// The normal fixture runner auto-generates models, not relationships. Supply
// the explicit relationship here, while using the live connector contract.
func stageEngineMetadata(t *testing.T, workdir, fixture string, schema, caps json.RawMessage) {
	t.Helper()
	var definitions struct {
		Subgraphs []struct {
			Objects []struct {
				Kind       string          `json:"kind"`
				Version    string          `json:"version"`
				Definition json.RawMessage `json:"definition"`
			} `json:"objects"`
			Name string `json:"name"`
		} `json:"subgraphs"`
		Version    string          `json:"version"`
		Supergraph json.RawMessage `json:"supergraph"`
	}
	check(t, "decode relationship metadata", json.Unmarshal([]byte(read(t, filepath.Join(fixture, "metadata.json"))), &definitions))
	for _, subgraph := range definitions.Subgraphs {
		for i, object := range subgraph.Objects {
			if object.Kind != "DataConnectorLink" {
				continue
			}
			var link map[string]json.RawMessage
			check(t, "decode connector link", json.Unmarshal(object.Definition, &link))
			contract, err := json.Marshal(map[string]any{"version": "v0.2", "schema": schema, "capabilities": caps})
			check(t, "encode connector contract", err)
			link["schema"] = contract
			subgraph.Objects[i].Definition, err = json.Marshal(link)
			check(t, "encode connector link", err)
		}
	}
	out, err := json.Marshal(definitions)
	check(t, "encode engine metadata", err)
	check(t, "write engine metadata", os.WriteFile(filepath.Join(workdir, "engine", "metadata.json"), out, 0o644))
	const auth = `{"version":"v1","definition":{"allowRoleEmulationBy":"admin","mode":{"webhook":{"url":"http://auth_hook:3060/validate-request","method":"Post"}}}}`
	check(t, "write local auth config", os.WriteFile(filepath.Join(workdir, "engine", "auth_config.json"), []byte(auth), 0o644))
}

func ndcJSON(t *testing.T, ctx context.Context, url, body string) json.RawMessage {
	t.Helper()
	method := http.MethodGet
	if body != "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	check(t, "create NDC request", err)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	check(t, "send NDC request", err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	check(t, "read NDC response", err)
	if resp.StatusCode != http.StatusOK || !json.Valid(data) {
		t.Fatalf("%s %s: status %d, body %s", method, url, resp.StatusCode, data)
	}
	return data
}

func read(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	check(t, "read "+name, err)
	return string(data)
}

func check(t *testing.T, operation string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", operation, err)
	}
}
