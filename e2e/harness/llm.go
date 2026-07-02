//go:build e2e

package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Verdict is the structured result of an LLM equivalence comparison.
type Verdict struct {
	Equivalent bool     `json:"equivalent"`
	Rationale  string   `json:"rationale"`
	Diffs      []string `json:"diffs"`
}

// CompareEquivalent asks the LLM whether two payloads represent the same result
// set, ignoring formatting and ordering. Per decision #3 of the spec we do NOT
// implement manual normalization/sorting/extraction — the model does the
// semantic comparison and returns a structured verdict.
//
// labelA/labelB describe the payloads (e.g. "DDN GraphQL result",
// "Elasticsearch _search result") so the model understands their differing
// shapes (GraphQL selection sets vs raw ES hits).
func CompareEquivalent(ctx context.Context, env *Env, description, labelA string, payloadA []byte, labelB string, payloadB []byte) (*Verdict, error) {
	system := strings.TrimSpace(`
You are a strict test oracle for a database connector's end-to-end tests. You are
given two JSON payloads produced by two different systems for the SAME logical
query. Decide whether they represent the SAME RESULT SET.

Rules:
- IGNORE ordering of array/list elements unless the query explicitly requested a
  sort; if a sort was requested, ordering MUST match.
- IGNORE differences that are purely structural/formatting: e.g. a GraphQL
  response nests rows under data.<model>, while a raw Elasticsearch response
  nests documents under hits.hits[]._source. Compare the underlying field values
  and the set of returned records/aggregations.
- IGNORE fields that one side simply did not select. Only compare the fields that
  BOTH payloads are expected to contain for this query. If one payload contains a
  superset of fields (e.g. ES returns the whole _source), restrict the comparison
  to the fields present on the other side.
- Numbers that are equal in value are equal even if one is a string and the other
  a number, or if one has trailing zeros.
- For aggregations, compare the aggregated values (counts, sums, avdetails), not
  the surrounding envelope.
- If a payload contains a GraphQL/ES ERROR, it is NOT equivalent.

Respond with ONLY a single JSON object, no prose, no code fences:
{"equivalent": <true|false>, "rationale": "<one or two sentences>", "diffs": ["<specific discrepancy>", ...]}
diffs must be empty when equivalent is true.`)

	user := fmt.Sprintf("Query context: %s\n\n=== %s ===\n%s\n\n=== %s ===\n%s",
		description, labelA, string(compact(payloadA)), labelB, string(compact(payloadB)))

	text, err := anthropicMessage(ctx, env, system, user)
	if err != nil {
		return nil, err
	}
	v, err := parseVerdict(text)
	if err != nil {
		return nil, fmt.Errorf("parsing LLM verdict: %w\nraw: %s", err, tail(text, 600))
	}
	return v, nil
}

// anthropicMessage calls the Anthropic-compatible messages API and returns the
// assistant's text.
func anthropicMessage(ctx context.Context, env *Env, system, user string) (string, error) {
	reqBody := map[string]interface{}{
		"model":      env.LLMModel,
		"max_tokens": 1024,
		"system":     system,
		"messages": []map[string]interface{}{
			{"role": "user", "content": user},
		},
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, env.LLMBaseURL, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", env.LLMAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API status %d: %s", resp.StatusCode, tail(string(raw), 600))
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decoding LLM response: %w", err)
	}
	var sb strings.Builder
	for _, c := range parsed.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String(), nil
}

// parseVerdict extracts the JSON verdict object from the model's text, tolerating
// stray code fences or leading/trailing prose.
func parseVerdict(text string) (*Verdict, error) {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end < 0 || end < start {
		return nil, fmt.Errorf("no JSON object found")
	}
	var v Verdict
	if err := json.Unmarshal([]byte(s[start:end+1]), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// compact re-marshals JSON compactly; on failure returns the input unchanged.
func compact(b []byte) []byte {
	var out bytes.Buffer
	if err := json.Compact(&out, b); err != nil {
		return b
	}
	return out.Bytes()
}
