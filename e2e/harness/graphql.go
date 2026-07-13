//go:build e2e

package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GraphQLResult holds the decoded response from the DDN engine.
type GraphQLResult struct {
	Data   json.RawMessage   `json:"data"`
	Errors []json.RawMessage `json:"errors"`
	Raw    []byte            `json:"-"`
}

// RunGraphQL POSTs a query (+ optional variables) to the DDN engine's /graphql
// endpoint as the admin role and returns the decoded result.
func RunGraphQL(ctx context.Context, engineURL, query string, variables []byte) (*GraphQLResult, error) {
	payload := map[string]interface{}{"query": query}
	if len(variables) > 0 {
		var vars interface{}
		if err := json.Unmarshal(variables, &vars); err != nil {
			return nil, fmt.Errorf("invalid variables.json: %w", err)
		}
		payload["variables"] = vars
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, engineURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// The dev auth webhook grants whatever role is requested; use admin so model
	// permissions added by `ddn model add` apply.
	req.Header.Set("x-hasura-role", "admin")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out GraphQLResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decoding graphql response (status %d): %w\nbody: %s", resp.StatusCode, err, tail(string(raw), 800))
	}
	out.Raw = raw
	if len(out.Errors) > 0 {
		return &out, fmt.Errorf("graphql returned errors: %s", tail(string(raw), 800))
	}
	return &out, nil
}
