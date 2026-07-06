//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	elasticPassword = "e2e-elastic-pass"
	kibanaPassword  = "e2e-kibana-pass"
)

// Stack manages one per-case docker compose project: bring-up, staged startup,
// port bookkeeping and teardown.
type Stack struct {
	env   *Env
	case_ Case

	Project string         // compose project name, e.g. e2e-kibana_sample_logs
	Workdir string         // host dir bind-mounted into connector/engine
	Ports   map[string]int // logical name -> published host port
	CACert  string         // host path to extracted ca.crt

	timings []StepTiming // per-step durations captured during the run
}

// NewStack initializes bookkeeping (does not touch docker yet).
func NewStack(env *Env, c Case) (*Stack, error) {
	off := portOffset(c.Name)
	workdir, err := os.MkdirTemp("", "e2e-"+c.Name+"-")
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"connector", "engine", "certs"} {
		if err := os.MkdirAll(filepath.Join(workdir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &Stack{
		env:     env,
		case_:   c,
		Project: "e2e-" + sanitizeProject(c.Name),
		Workdir: workdir,
		Ports: map[string]int{
			"es":        19200 + off,
			"kibana":    15601 + off,
			"connector": 18080 + off,
			"engine":    13000 + off,
			"auth":      13050 + off,
		},
		CACert: filepath.Join(workdir, "certs", "ca.crt"),
	}, nil
}

// composeEnv returns the environment the compose CLI needs for variable
// substitution in docker-compose.e2e.yaml.
func (s *Stack) composeEnv() []string {
	return []string{
		"STACK_VERSION=" + s.env.StackVersion,
		"ELASTIC_PASSWORD=" + elasticPassword,
		"KIBANA_PASSWORD=" + kibanaPassword,
		"CASE_WORKDIR=" + s.Workdir,
		"ES_PORT=" + strconv.Itoa(s.Ports["es"]),
		"KIBANA_PORT=" + strconv.Itoa(s.Ports["kibana"]),
		"CONNECTOR_PORT=" + strconv.Itoa(s.Ports["connector"]),
		"ENGINE_PORT=" + strconv.Itoa(s.Ports["engine"]),
		"AUTH_PORT=" + strconv.Itoa(s.Ports["auth"]),
	}
}

func (s *Stack) compose(args ...string) []string {
	base := []string{"compose", "-f", s.env.ComposeFile, "-p", s.Project}
	return append(base, args...)
}

// StartElasticsearch brings up the cert bootstrap + ES, then waits for ES to
// authenticate. Kibana (for sample-data cases) is started separately via
// StartKibana, after the kibana_system password has been set.
func (s *Stack) StartElasticsearch(ctx context.Context) (*ESClient, error) {
	t := s.startTimer("stack:elasticsearch-up")
	// `setup` (the one-shot cert bootstrap) is pulled in automatically via es01's
	// depends_on: service_completed_successfully, so we only --wait on es01
	// (waiting on a service that exits can confuse `--wait`).
	args := s.compose("up", "-d", "--wait", "es01")
	if _, err := mustRun(ctx, s.env.E2EDir, s.composeEnv(), "docker", args...); err != nil {
		t.done()
		return nil, fmt.Errorf("bringing up elasticsearch: %w", err)
	}
	if err := s.extractCACert(ctx); err != nil {
		t.done()
		return nil, err
	}
	client, err := NewESClient(
		fmt.Sprintf("https://localhost:%d", s.Ports["es"]),
		"elastic", elasticPassword, s.CACert,
	)
	if err != nil {
		t.done()
		return nil, err
	}
	if err := client.WaitReady(ctx); err != nil {
		t.done()
		return nil, err
	}
	t.done()
	return client, nil
}

// extractCACert copies ca.crt out of the running es01 container to the host so
// the harness's ES/HTTPS clients can trust the self-signed stack.
func (s *Stack) extractCACert(ctx context.Context) error {
	res, err := mustRun(ctx, s.env.E2EDir, s.composeEnv(), "docker",
		s.compose("exec", "-T", "es01", "cat", "config/certs/ca/ca.crt")...)
	if err != nil {
		return fmt.Errorf("extracting CA cert: %w", err)
	}
	if err := os.WriteFile(s.CACert, []byte(res.Stdout), 0o644); err != nil {
		return err
	}
	return nil
}

// StartKibana sets the kibana_system password (via the elastic superuser) and
// then brings up kibana, waiting for it to become healthy. Only used for
// kibana-sample cases.
func (s *Stack) StartKibana(ctx context.Context, es *ESClient) error {
	t := s.startTimer("stack:kibana-up")
	defer t.done()
	if err := es.SetKibanaSystemPassword(ctx, kibanaPassword); err != nil {
		return fmt.Errorf("setting kibana_system password: %w", err)
	}
	if _, err := mustRun(ctx, s.env.E2EDir, s.composeEnv(), "docker",
		s.compose("--profile", "kibana", "up", "-d", "--wait", "kibana")...); err != nil {
		return fmt.Errorf("bringing up kibana: %w", err)
	}
	return waitHTTP(ctx, fmt.Sprintf("http://localhost:%d/api/status", s.Ports["kibana"]), 3*time.Minute)
}

// KibanaBaseURL returns the host URL for the kibana instance.
func (s *Stack) KibanaBaseURL() string {
	return fmt.Sprintf("http://localhost:%d", s.Ports["kibana"])
}

// Introspect runs the connector's `update` command as a one-shot container,
// writing ${CASE_WORKDIR}/connector/configuration.json from the live ES.
// For Kibana sample-data cases the raw config lists system and backing-index
// names that are not usable as GraphQL identifiers; filterKibanaConfig rewrites
// it to expose only the data-stream alias as a single named collection.
func (s *Stack) Introspect(ctx context.Context) error {
	t := s.startTimer("stack:introspect")
	defer t.done()
	_, err := mustRun(ctx, s.env.E2EDir, s.composeEnv(), "docker",
		s.compose("--profile", "tools", "run", "--rm", "introspect")...)
	if err != nil {
		return fmt.Errorf("connector introspection (update) failed: %w", err)
	}
	cfg := filepath.Join(s.Workdir, "connector", "configuration.json")
	if _, err := os.Stat(cfg); err != nil {
		return fmt.Errorf("introspection did not produce %s: %w", cfg, err)
	}
	if s.case_.Meta.KibanaSample != "" {
		if err := filterKibanaConfig(cfg, s.case_.Meta.KibanaSample); err != nil {
			return fmt.Errorf("filtering kibana config: %w", err)
		}
	}
	return nil
}

// filterKibanaConfig rewrites configuration.json so that it contains only one
// index entry keyed by the data-stream alias (kibana_sample_data_<kind>).
// Kibana stores sample data in a data stream whose backing index has a
// date-stamped name like .ds-kibana_sample_data_logs-2026.07.06-000001.
// The connector's update command discovers that physical name, which cannot
// be used as a GraphQL identifier. We find it by its prefix, rename it to the
// alias, and drop all other indices so `ddn model add` only creates one model.
func filterKibanaConfig(cfgPath, kibanaSampleKind string) error {
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	alias := "kibana_sample_data_" + kibanaSampleKind // e.g. kibana_sample_data_logs
	backingPrefix := ".ds-" + alias                   // e.g. .ds-kibana_sample_data_logs

	indices, _ := cfg["indices"].(map[string]interface{})
	var aliasMapping interface{}
	for name, mapping := range indices {
		if strings.HasPrefix(name, backingPrefix) || name == alias {
			aliasMapping = mapping
			break
		}
	}
	if aliasMapping == nil {
		return fmt.Errorf("could not find a backing index for %s in configuration.json", alias)
	}
	cfg["indices"] = map[string]interface{}{alias: aliasMapping}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, out, 0o644)
}

// StartConnector brings up the connector in serve mode and waits for /health.
func (s *Stack) StartConnector(ctx context.Context) error {
	t := s.startTimer("stack:connector-up")
	defer t.done()
	if _, err := mustRun(ctx, s.env.E2EDir, s.composeEnv(), "docker",
		s.compose("up", "-d", "--wait", "ndc-elasticsearch")...); err != nil {
		return fmt.Errorf("bringing up connector: %w", err)
	}
	return waitHTTP(ctx, fmt.Sprintf("http://localhost:%d/health", s.Ports["connector"]), 90*time.Second)
}

// StartEngine brings up auth_hook + engine and waits for the GraphQL endpoint.
func (s *Stack) StartEngine(ctx context.Context) error {
	t := s.startTimer("stack:engine-up")
	defer t.done()
	if _, err := mustRun(ctx, s.env.E2EDir, s.composeEnv(), "docker",
		s.compose("--profile", "engine", "up", "-d", "auth_hook", "engine")...); err != nil {
		return fmt.Errorf("bringing up engine: %w", err)
	}
	// The engine serves GraphQL at /graphql; a GET returns 200/400 once ready.
	return waitHTTP(ctx, fmt.Sprintf("http://localhost:%d/graphql", s.Ports["engine"]), 90*time.Second)
}

// Down tears the stack down (including volumes) unless KEEP_STACK=1.
func (s *Stack) Down(ctx context.Context) {
	if s.env.KeepStack {
		fmt.Printf("[%s] KEEP_STACK=1 -> leaving stack up (project=%s, workdir=%s)\n",
			s.case_.Name, s.Project, s.Workdir)
		return
	}
	// Best-effort: use a fresh context so teardown still runs if ctx was cancelled.
	downCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_ = run(downCtx, s.env.E2EDir, s.composeEnv(), "docker",
		s.compose("--profile", "kibana", "--profile", "engine", "--profile", "tools",
			"down", "-v", "--remove-orphans")...)
	_ = os.RemoveAll(s.Workdir)
}

// EngineGraphQLURL / ESBaseURL helpers.
func (s *Stack) EngineGraphQLURL() string {
	return fmt.Sprintf("http://localhost:%d/graphql", s.Ports["engine"])
}

func (s *Stack) startTimer(step string) *stepTimer {
	return &stepTimer{stack: s, step: step, start: time.Now()}
}

// Timings returns all captured step timings for the report.
func (s *Stack) Timings() []StepTiming { return s.timings }

type stepTimer struct {
	stack *Stack
	step  string
	start time.Time
}

func (t *stepTimer) done() {
	t.stack.timings = append(t.stack.timings, StepTiming{
		Step:       t.step,
		DurationMS: time.Since(t.start).Milliseconds(),
	})
}

// waitHTTP polls a URL until it responds with any HTTP status (connection
// succeeds) or the timeout elapses.
func waitHTTP(ctx context.Context, url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		last = err
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timed out waiting for %s: %v", url, last)
}

// sanitizeProject makes a compose-safe project name (lowercase, no dots).
func sanitizeProject(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}
