//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

// testEnv is populated by TestMain when E2E=1.
var (
	testEnv      *Env
	globalReport = &Report{Env: map[string]string{}}
	reportMu     sync.Mutex
)

// TestMain loads configuration once and always writes the report afterwards.
func TestMain(m *testing.M) {
	if os.Getenv("E2E") == "1" {
		env, err := LoadEnv()
		if err != nil {
			fmt.Fprintln(os.Stderr, "e2e configuration error:", err)
			os.Exit(1)
		}
		testEnv = env
		globalReport.StartedAt = time.Now().Format(time.RFC3339)
		globalReport.Env = map[string]string{
			"UPDATE_GOLDEN": strconv.FormatBool(env.UpdateGolden),
			"FAIL_FAST":     strconv.FormatBool(env.FailFast),
			"STACK_VERSION": env.StackVersion,
			"LLM_MODEL":     env.LLMModel,
		}
	}

	code := m.Run()

	if testEnv != nil {
		globalReport.FinishedAt = time.Now().Format(time.RFC3339)
		globalReport.Finalize()
		jsonPath, mdPath, err := globalReport.WriteFiles(testEnv.ReportDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to write e2e report:", err)
		} else {
			fmt.Printf("\ne2e report written:\n  %s\n  %s\n", jsonPath, mdPath)
		}
	}
	os.Exit(code)
}

// TestE2E discovers every case under e2e/cases and runs it as a subtest. Adding
// a new case is purely a matter of dropping files in — this runner is never
// edited (spec requirement).
func TestE2E(t *testing.T) {
	if os.Getenv("E2E") != "1" {
		t.Skip("e2e suite is gated: run with E2E=1 (and the `e2e` build tag)")
	}
	requireCmd(t, "docker")

	cases, err := DiscoverCases(testEnv)
	if err != nil {
		t.Fatalf("discovering cases: %v", err)
	}
	if len(cases) == 0 {
		t.Fatalf("no cases discovered under %s", testEnv.CasesDir)
	}
	t.Logf("discovered %d case(s)", len(cases))

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			runCase(t, testEnv, c)
		})
	}
}

// runCase provisions a fresh isolated stack for a single case, runs L3 + L4, and
// records a CaseReport regardless of outcome.
func runCase(t *testing.T, env *Env, c Case) {
	start := time.Now()
	cr := CaseReport{
		Name:        c.Name,
		SchemaLayer: "L3",
		Status:      StatusPass,
	}
	if c.Meta.Description != "" {
		cr.Message = c.Meta.Description
	}

	stack, err := NewStack(env, c)
	if err != nil {
		cr.Status = StatusFail
		cr.Message = "stack init: " + err.Error()
		recordCase(&cr, start, nil)
		t.Fatalf("stack init: %v", err)
	}

	// Always tear down and always record the case report.
	defer func() {
		recordCase(&cr, start, stack)
		stack.Down(context.Background())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	t.Logf("[%s] bringing up Elasticsearch (project=%s, es port=%d)", c.Name, stack.Project, stack.Ports["es"])
	es, err := stack.StartElasticsearch(ctx)
	if err != nil {
		fail(t, &cr, "elasticsearch startup: "+err.Error())
		return
	}

	t.Logf("[%s] seeding", c.Name)
	if err := Seed(ctx, stack, es, c); err != nil {
		fail(t, &cr, "seeding: "+err.Error())
		return
	}

	t.Logf("[%s] introspecting connector configuration", c.Name)
	if err := stack.Introspect(ctx); err != nil {
		fail(t, &cr, "introspection: "+err.Error())
		return
	}

	t.Logf("[%s] starting connector", c.Name)
	if err := stack.StartConnector(ctx); err != nil {
		fail(t, &cr, "connector startup: "+err.Error())
		return
	}

	// ---- L3: schema conformance ----
	t.Run("L3-schema", func(t *testing.T) {
		problems, err := AssertSchemaConformance(ctx, stack, es)
		if err != nil {
			cr.SchemaStatus = StatusFail
			cr.Status = StatusFail
			cr.SchemaProblems = []string{"assertion error: " + err.Error()}
			t.Fatalf("[%s] L3 schema assertion error: %v", c.Name, err)
		}
		if len(problems) > 0 {
			cr.SchemaStatus = StatusFail
			cr.Status = StatusFail
			cr.SchemaProblems = problems
			t.Errorf("[%s] L3 schema conformance: %d problem(s):\n- %s",
				c.Name, len(problems), joinLines(problems))
			return
		}
		cr.SchemaStatus = StatusPass
	})

	// ---- build DDN + start engine ----
	t.Logf("[%s] building DDN metadata via ddn CLI", c.Name)
	if err := BuildDDN(ctx, stack, c); err != nil {
		fail(t, &cr, "ddn build: "+err.Error())
		return
	}
	t.Logf("[%s] starting DDN engine", c.Name)
	if err := stack.StartEngine(ctx); err != nil {
		fail(t, &cr, "engine startup: "+err.Error())
		return
	}

	// ---- L4: query parity + goldens ----
	for _, q := range c.Queries {
		q := q
		t.Run("L4-"+q.Name, func(t *testing.T) {
			qr := runQuery(ctx, t, env, stack, es, c, q)
			cr.Queries = append(cr.Queries, qr)
			if qr.Status == StatusFail {
				cr.Status = StatusFail
			}
		})
	}
}

// runQuery executes one L4 query: raw ES _search + DDN GraphQL, then either
// (re)generates goldens or LLM-compares DDN-vs-ES and each-vs-its-golden.
func runQuery(ctx context.Context, t *testing.T, env *Env, stack *Stack, es *ESClient, c Case, q Query) QueryReport {
	qStart := time.Now()
	qr := QueryReport{Name: q.Name, Layer: "L4", Target: q.Target.Index}
	desc := q.Target.Description
	if desc == "" {
		desc = fmt.Sprintf("case=%s query=%s target=%s", c.Name, q.Name, q.Target.Index)
	}

	// Direct ES search.
	esBody, err := es.Search(ctx, q.Target.Index, q.ESSearch)
	if err != nil {
		qr.Status = StatusFail
		qr.Message = "ES _search: " + err.Error()
		qr.ESPayload = string(esBody)
		t.Errorf("[%s/%s] ES search failed: %v", c.Name, q.Name, err)
		qr.Timings = []StepTiming{{Step: "query", DurationMS: time.Since(qStart).Milliseconds()}}
		return qr
	}
	qr.ESPayload = pretty(esBody)

	// DDN GraphQL.
	ddnRes, err := RunGraphQL(ctx, stack.EngineGraphQLURL(), q.GraphQL, q.Variables)
	if err != nil {
		qr.Status = StatusFail
		qr.Message = "DDN GraphQL: " + err.Error()
		if ddnRes != nil {
			qr.DDNPayload = pretty(ddnRes.Raw)
		}
		t.Errorf("[%s/%s] DDN query failed: %v", c.Name, q.Name, err)
		qr.Timings = []StepTiming{{Step: "query", DurationMS: time.Since(qStart).Milliseconds()}}
		return qr
	}
	ddnData := []byte(ddnRes.Data)
	qr.DDNPayload = pretty(ddnData)

	// Golden regeneration mode: write goldens and pass.
	if env.UpdateGolden {
		if err := os.WriteFile(q.GoldenDDNPath, []byte(pretty(ddnData)), 0o644); err != nil {
			qr.Status = StatusFail
			qr.Message = "writing golden.ddn.json: " + err.Error()
			t.Errorf("%v", err)
			return qr
		}
		if err := os.WriteFile(q.GoldenESPath, []byte(pretty(esBody)), 0o644); err != nil {
			qr.Status = StatusFail
			qr.Message = "writing golden.es.json: " + err.Error()
			t.Errorf("%v", err)
			return qr
		}
		qr.Status = StatusPass
		qr.Message = "goldens regenerated (UPDATE_GOLDEN=1)"
		t.Logf("[%s/%s] goldens regenerated", c.Name, q.Name)
		qr.Timings = []StepTiming{{Step: "query", DurationMS: time.Since(qStart).Milliseconds()}}
		return qr
	}

	// Comparison mode.
	goldenDDN, _ := os.ReadFile(q.GoldenDDNPath)
	goldenES, _ := os.ReadFile(q.GoldenESPath)
	qr.GoldenDDN = string(goldenDDN)
	qr.GoldenES = string(goldenES)

	comparisons := []struct {
		name           string
		labelA, labelB string
		a, b           []byte
	}{
		{"ddn-vs-es", "DDN GraphQL result", "Elasticsearch _search result", ddnData, esBody},
		{"ddn-vs-golden", "DDN GraphQL result", "golden.ddn.json (expected)", ddnData, goldenDDN},
		{"es-vs-golden", "Elasticsearch _search result", "golden.es.json (expected)", esBody, goldenES},
	}

	allEquivalent := true
	for _, cmp := range comparisons {
		nv := NamedVerdict{Comparison: cmp.name}
		isGoldenCmp := cmp.name == "ddn-vs-golden" || cmp.name == "es-vs-golden"
		if isGoldenCmp && len(cmp.b) == 0 {
			nv.Error = "golden file missing; run with UPDATE_GOLDEN=1 to create it"
			allEquivalent = false
			qr.Verdicts = append(qr.Verdicts, nv)
			continue
		}
		// A "pending" golden (sentinel {"__pending__": true}) is committed for
		// cases whose values are environment-derived (e.g. the kibana sample
		// dataset). Its golden comparison is SKIPPED — the live DDN-vs-ES parity
		// check still decides pass/fail — and the author regenerates it with
		// UPDATE_GOLDEN=1. See e2e/README.md.
		if isGoldenCmp && isPendingGolden(cmp.b) {
			nv.Error = "golden pending regeneration (UPDATE_GOLDEN=1); comparison skipped"
			qr.Verdicts = append(qr.Verdicts, nv)
			continue
		}
		v, err := CompareEquivalent(ctx, env, desc, cmp.labelA, cmp.a, cmp.labelB, cmp.b)
		if err != nil {
			nv.Error = err.Error()
			allEquivalent = false
		} else {
			nv.Verdict = v
			if !v.Equivalent {
				allEquivalent = false
			}
		}
		qr.Verdicts = append(qr.Verdicts, nv)
	}

	if allEquivalent {
		qr.Status = StatusPass
	} else {
		qr.Status = StatusFail
		t.Errorf("[%s/%s] L4 parity failed; see report for LLM verdicts and payloads", c.Name, q.Name)
	}
	qr.Timings = []StepTiming{{Step: "query", DurationMS: time.Since(qStart).Milliseconds()}}
	return qr
}

// ---- helpers ----

func fail(t *testing.T, cr *CaseReport, msg string) {
	cr.Status = StatusFail
	if cr.Message == "" {
		cr.Message = msg
	} else {
		cr.Message = cr.Message + "; " + msg
	}
	t.Errorf("%s: %s", cr.Name, msg)
}

func recordCase(cr *CaseReport, start time.Time, stack *Stack) {
	cr.DurationMS = time.Since(start).Milliseconds()
	if stack != nil {
		cr.Timings = stack.Timings()
	}
	reportMu.Lock()
	globalReport.Cases = append(globalReport.Cases, *cr)
	reportMu.Unlock()
}

func requireCmd(t *testing.T, name string) {
	if !commandExists(name) {
		t.Fatalf("required command %q not found on PATH", name)
	}
}

// isPendingGolden reports whether a golden file is the pending-regeneration
// sentinel {"__pending__": true}.
func isPendingGolden(b []byte) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return false
	}
	v, ok := m["__pending__"].(bool)
	return ok && v
}

func pretty(b []byte) string {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(b)
	}
	return string(out)
}

func joinLines(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += "\n- "
		}
		out += s
	}
	return out
}
