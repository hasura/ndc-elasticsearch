//go:build e2e

package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"
)

// schemaGoldenResult is the outcome of the case-level schema-golden step.
type schemaGoldenResult struct {
	Status   string // StatusPass | StatusFail | StatusSkip
	Message  string
	Actual   string // canonicalized connector /schema (attached on failure)
	Expected string // canonicalized golden.schema.json (attached on failure)
}

// FetchConnectorSchemaRaw GETs the connector's full /schema body (unlike
// FetchConnectorSchema, which decodes only a minimal projection for L3).
func FetchConnectorSchemaRaw(ctx context.Context, connectorPort int) ([]byte, error) {
	url := fmt.Sprintf("http://localhost:%d/schema", connectorPort)
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s => %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// canonicalizeSchema returns a stable, pretty-printed /schema. The connector
// emits collections/functions/procedures in nondeterministic Go-map order, so
// sort those arrays by name (object keys are already sorted by the encoder) to
// keep the golden stable across runs.
func canonicalizeSchema(raw []byte) ([]byte, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing schema JSON: %w", err)
	}
	for _, key := range []string{"collections", "functions", "procedures"} {
		arr, ok := doc[key].([]interface{})
		if !ok {
			continue
		}
		sort.SliceStable(arr, func(i, j int) bool {
			return jsonObjectName(arr[i]) < jsonObjectName(arr[j])
		})
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return out, nil
}

// isPendingGolden reports whether a golden file is the pending-regeneration
// sentinel {"__pending__": true}. Shared by the query goldens (e2e_test.go) and
// the schema golden below.
func isPendingGolden(b []byte) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return false
	}
	v, ok := m["__pending__"].(bool)
	return ok && v
}

// jsonObjectName extracts the "name" field of a decoded JSON object, or "" if
// absent / not an object.
func jsonObjectName(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := m["name"].(string)
	return name
}

// AssertSchemaGolden snapshots the connector's full /schema and either
// regenerates golden.schema.json (UPDATE_GOLDEN=1) or compares against it, so a
// change to the generated schema surface shows up as a diff rather than
// passing silently.
func AssertSchemaGolden(ctx context.Context, env *Env, s *Stack, c Case) (schemaGoldenResult, error) {
	raw, err := FetchConnectorSchemaRaw(ctx, s.Ports["connector"])
	if err != nil {
		return schemaGoldenResult{}, err
	}
	canonical, err := canonicalizeSchema(raw)
	if err != nil {
		return schemaGoldenResult{}, err
	}
	res := schemaGoldenResult{Actual: string(canonical)}

	// Regeneration mode: write the golden and pass.
	if env.UpdateGolden {
		if err := os.WriteFile(c.GoldenSchemaPath, append(canonical, '\n'), 0o644); err != nil {
			return res, fmt.Errorf("writing golden.schema.json: %w", err)
		}
		res.Status = StatusPass
		res.Message = "schema golden regenerated (UPDATE_GOLDEN=1)"
		return res, nil
	}

	// Comparison mode.
	golden, _ := os.ReadFile(c.GoldenSchemaPath)
	switch {
	case len(golden) == 0:
		res.Status = StatusFail
		res.Message = "golden.schema.json missing; run with UPDATE_GOLDEN=1"
		res.Expected = string(golden)
		return res, nil
	case isPendingGolden(golden):
		res.Status = StatusSkip
		res.Message = "golden.schema.json is pending; run UPDATE_GOLDEN=1 to capture it"
		return res, nil
	}

	// Re-canonicalize the golden so formatting/order differences don't mismatch.
	goldenCanonical, err := canonicalizeSchema(golden)
	if err != nil {
		res.Status = StatusFail
		res.Message = "golden.schema.json is not valid JSON: " + err.Error()
		res.Expected = string(golden)
		return res, nil
	}
	res.Expected = string(goldenCanonical)

	if !bytes.Equal(canonical, goldenCanonical) {
		res.Status = StatusFail
		res.Message = "connector /schema does not match golden.schema.json (run UPDATE_GOLDEN=1 to review/update)"
		return res, nil
	}
	res.Status = StatusPass
	return res, nil
}
