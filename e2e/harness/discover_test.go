//go:build e2e

package harness

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// TestDiscoverCases is a fast, docker-free structural check that every committed
// case parses: queries load, required files are present, es_search.json and the
// goldens are valid JSON, and target.yaml sets an index. It runs under
// `go test -tags e2e` regardless of E2E=1, giving quick feedback while authoring
// cases without provisioning any stack.
func TestDiscoverCases(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repoRoot: %v", err)
	}
	env := &Env{CasesDir: filepath.Join(root, "e2e", "cases")}

	cases, err := DiscoverCases(env)
	if err != nil {
		t.Fatalf("DiscoverCases: %v", err)
	}
	if len(cases) < 2 {
		t.Fatalf("expected at least 2 cases (kibana_sample_logs + custom_products), got %d", len(cases))
	}

	for _, c := range cases {
		if len(c.Queries) == 0 {
			t.Errorf("case %q: no queries", c.Name)
		}
		for _, q := range c.Queries {
			if q.GraphQL == "" {
				t.Errorf("%s/%s: empty query.graphql", c.Name, q.Name)
			}
			if q.Target.Index == "" {
				t.Errorf("%s/%s: target.yaml missing index", c.Name, q.Name)
			}
			var v interface{}
			if err := json.Unmarshal(q.ESSearch, &v); err != nil {
				t.Errorf("%s/%s: es_search.json invalid JSON: %v", c.Name, q.Name, err)
			}
			if len(q.Variables) > 0 {
				if err := json.Unmarshal(q.Variables, &v); err != nil {
					t.Errorf("%s/%s: variables.json invalid JSON: %v", c.Name, q.Name, err)
				}
			}
		}
	}
	t.Logf("discovered %d cases", len(cases))
}
