//go:build e2e

package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Env is the set of knobs that control a single e2e run. Everything is sourced
// from environment variables so the harness behaves identically whether it is
// invoked from the Makefile locally or from CI.
type Env struct {
	// UpdateGolden, when true, regenerates golden.ddn.json / golden.es.json for
	// every query instead of comparing against them. (UPDATE_GOLDEN=1)
	UpdateGolden bool

	// FailFast maps to `go test -failfast`; this field is informational only
	// (the Makefile passes -failfast to `go test`). Reported for visibility.
	FailFast bool

	// KeepStack, when true, leaves the per-case docker compose project running
	// after the case finishes. Useful for debugging. (KEEP_STACK=1)
	KeepStack bool

	// CaseFilter, when non-empty, restricts the run to a single case directory
	// name. Set by `make e2e-case CASE=<name>` via E2E_CASE. Empty => all cases.
	CaseFilter string

	// Paths, resolved once at startup.
	RepoRoot    string // absolute path to the repository root
	E2EDir      string // <repo>/e2e
	CasesDir    string // <repo>/e2e/cases
	ComposeFile string // <repo>/e2e/docker-compose.e2e.yaml
	ReportDir   string // <repo>/e2e/report (written each run)

	// StackVersion pins the Elasticsearch / Kibana / connector stack. (STACK_VERSION)
	StackVersion string
}

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// LoadEnv reads all configuration from the process environment and resolves the
// repository-relative paths.
func LoadEnv() (*Env, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}

	e := &Env{
		UpdateGolden: envBool("UPDATE_GOLDEN"),
		FailFast:     envBool("FAIL_FAST"),
		KeepStack:    envBool("KEEP_STACK"),
		CaseFilter:   strings.TrimSpace(os.Getenv("E2E_CASE")),
		StackVersion: envOr("STACK_VERSION", "8.13.4"),
		RepoRoot:     root,
	}
	e.E2EDir = filepath.Join(root, "e2e")
	e.CasesDir = filepath.Join(e.E2EDir, "cases")
	e.ComposeFile = filepath.Join(e.E2EDir, "docker-compose.e2e.yaml")
	e.ReportDir = filepath.Join(e.E2EDir, "report")

	return e, nil
}

// repoRoot walks up from the current working directory looking for go.mod so the
// harness works regardless of which directory `go test` is invoked from.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate repository root (go.mod) from %s", dir)
		}
		dir = parent
	}
}

// portOffset derives a deterministic-ish port offset for a case so concurrent
// or leftover stacks are less likely to collide. It is a simple stable hash of
// the case name mapped into a small range; the compose project name still keeps
// stacks isolated, this only spreads published host ports.
func portOffset(caseName string) int {
	var h int
	for _, r := range caseName {
		h = (h*31 + int(r)) & 0x7fffffff
	}
	return h % 200 // 0..199
}

// atoiOr parses s as an int, returning def on failure.
func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}
