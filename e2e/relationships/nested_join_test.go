//go:build e2e

package relationships

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hasura/ndc-elasticsearch/e2e/harness"
)

// TestArrayRelationshipFromNestedObjects joins each object of a nested field to
// another index through an array relationship, in a single GraphQL operation.
// The source documents cover a nested field that is absent, null or empty, a
// nested object without the join key, and a mix of matched, unmatched and
// keyless objects in one document.
func TestArrayRelationshipFromNestedObjects(t *testing.T) {
	if os.Getenv("E2E") != "1" {
		t.Skip("run with E2E=1 and -tags e2e")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	env, err := harness.LoadEnv()
	check(t, "load e2e config", err)
	fixture := filepath.Join(env.E2EDir, "relationships", "testdata", "nested_join")
	c := harness.Case{
		Name: "nested_join_relationship",
		IndexMappings: []string{
			filepath.Join(fixture, "nested_coverage.mapping.json"),
			filepath.Join(fixture, "nested_hr.mapping.json"),
		},
		DataFiles: []string{filepath.Join(fixture, "documents.ndjson")},
	}
	stack, err := harness.NewStack(env, c)
	check(t, "create stack", err)
	t.Cleanup(func() { stack.Down(context.Background()) })

	es, err := stack.StartElasticsearch(ctx)
	check(t, "start Elasticsearch", err)
	check(t, "seed both indices", harness.Seed(ctx, stack, es, c))
	check(t, "introspect connector", stack.Introspect(ctx))
	check(t, "start connector", stack.StartConnector(ctx))
	connectorURL := fmt.Sprintf("http://localhost:%d", stack.Ports["connector"])
	schema := ndcJSON(t, ctx, connectorURL+"/schema", "")
	caps := ndcJSON(t, ctx, connectorURL+"/capabilities", "")
	stageEngineMetadata(t, stack.Workdir, fixture, schema, caps)
	check(t, "start DDN engine", stack.StartEngine(ctx))

	const (
		noOfficers = `[]`
		matched    = `[{"employeeId":"id-a","label":"matched","employeeHr":[{"display":"Alpha"}]}]`
		noMatch    = `[{"employeeId":"id-miss","label":"no-match","employeeHr":[]}]`
		noID       = `[{"employeeId":null,"label":"no-id","employeeHr":[]}]`
		mixed      = `[
			{"employeeId":"id-a","label":"a","employeeHr":[{"display":"Alpha"}]},
			{"employeeId":null,"label":"missing-key","employeeHr":[]},
			{"employeeId":null,"label":"null-key","employeeHr":[]},
			{"employeeId":"id-miss","label":"unmatched","employeeHr":[]},
			{"employeeId":"id-a","label":"duplicate","employeeHr":[{"display":"Alpha"}]},
			{"employeeId":"id-b","label":"b","employeeHr":[{"display":"Beta"}]}
		]`
	)

	// Every case is fetched in the same operation, so a failure to resolve one
	// document's join would fail the whole response.
	t.Run("single_operation", func(t *testing.T) {
		got := coverageByCase(t, ctx, stack.EngineGraphQLURL(),
			`{ coverage { caseId officers { employeeId label employeeHr { display } } } }`)
		assertOfficers(t, got, map[string]string{
			"missing":  noOfficers,
			"empty":    noOfficers,
			"null":     noOfficers,
			"matched":  matched,
			"no-match": noMatch,
			"no-id":    noID,
			"mixed":    mixed,
		})
	})

	// The nested projection alone, without the relationship.
	t.Run("projection_without_join", func(t *testing.T) {
		got := coverageByCase(t, ctx, stack.EngineGraphQLURL(),
			`{ coverage { caseId officers { employeeId label } } }`)
		assertOfficers(t, got, map[string]string{
			"missing":  noOfficers,
			"empty":    noOfficers,
			"null":     noOfficers,
			"matched":  `[{"employeeId":"id-a","label":"matched"}]`,
			"no-match": `[{"employeeId":"id-miss","label":"no-match"}]`,
			"no-id":    `[{"employeeId":null,"label":"no-id"}]`,
			"mixed": `[
				{"employeeId":"id-a","label":"a"},
				{"employeeId":null,"label":"missing-key"},
				{"employeeId":null,"label":"null-key"},
				{"employeeId":"id-miss","label":"unmatched"},
				{"employeeId":"id-a","label":"duplicate"},
				{"employeeId":"id-b","label":"b"}
			]`,
		})
	})

	// A document whose only officer lacks the join key, so the single variable
	// set sent for the relationship is null.
	t.Run("only_null_join_keys", func(t *testing.T) {
		res, err := harness.RunGraphQL(ctx, stack.EngineGraphQLURL(),
			`{ coverageById(caseId: "no-id") { caseId officers { employeeId label employeeHr { display } } } }`, nil)
		check(t, "query DDN GraphQL", err)
		var data struct {
			Coverage *struct {
				CaseID   string          `json:"caseId"`
				Officers json.RawMessage `json:"officers"`
			} `json:"coverageById"`
		}
		check(t, "decode DDN row", json.Unmarshal(res.Data, &data))
		if data.Coverage == nil {
			t.Fatalf("coverageById returned null: %s", res.Data)
		}
		assertOfficers(t, map[string]json.RawMessage{data.Coverage.CaseID: data.Coverage.Officers}, map[string]string{"no-id": noID})
	})
}

func coverageByCase(t *testing.T, ctx context.Context, url, query string) map[string]json.RawMessage {
	t.Helper()
	res, err := harness.RunGraphQL(ctx, url, query, nil)
	check(t, "query DDN GraphQL", err)
	var data struct {
		Coverage []struct {
			CaseID   string          `json:"caseId"`
			Officers json.RawMessage `json:"officers"`
		} `json:"coverage"`
	}
	check(t, "decode DDN rows", json.Unmarshal(res.Data, &data))
	byCase := make(map[string]json.RawMessage, len(data.Coverage))
	for _, row := range data.Coverage {
		byCase[row.CaseID] = row.Officers
	}
	return byCase
}

func assertOfficers(t *testing.T, got map[string]json.RawMessage, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("got %d coverage rows, want %d", len(got), len(want))
	}
	for caseID, expected := range want {
		actual, ok := got[caseID]
		if !ok {
			t.Errorf("case %q: missing from the response", caseID)
			continue
		}
		var gotValue, wantValue any
		check(t, "decode officers", json.Unmarshal(actual, &gotValue))
		check(t, "decode expected officers", json.Unmarshal([]byte(expected), &wantValue))
		if !reflect.DeepEqual(gotValue, wantValue) {
			t.Errorf("case %q officers:\n got  %s\n want %s", caseID, actual, expected)
		}
	}
}
