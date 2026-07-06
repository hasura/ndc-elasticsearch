//go:build e2e

package harness

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// runResult captures the outcome of an external command.
type runResult struct {
	Cmd      string
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	Err      error
}

// run executes an external command with `env` appended to the parent process
// environment (os.Environ). It never panics; the error (if any) is on the result.
func run(ctx context.Context, dir string, env []string, name string, args ...string) runResult {
	var full []string
	if len(env) > 0 {
		full = append(os.Environ(), env...)
	}
	return runFullEnv(ctx, dir, full, name, args...)
}

// runFullEnv is like run but sets the child's environment to fullEnv EXACTLY
// (no implicit merge with os.Environ). Pass nil to inherit the parent env. Use
// this when a command must run with a variable REMOVED from the environment.
func runFullEnv(ctx context.Context, dir string, fullEnv []string, name string, args ...string) runResult {
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if fullEnv != nil {
		cmd.Env = fullEnv
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := runResult{
		Cmd:      name + " " + strings.Join(args, " "),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
		Err:      err,
	}
	if ee, ok := err.(*exec.ExitError); ok {
		res.ExitCode = ee.ExitCode()
	} else if err != nil {
		res.ExitCode = -1
	}
	return res
}

// mustRun runs a command (env appended to os.Environ) and returns an error on failure.
func mustRun(ctx context.Context, dir string, env []string, name string, args ...string) (runResult, error) {
	return checkRun(run(ctx, dir, env, name, args...))
}

// mustRunFullEnv runs a command with an EXACT environment and returns an error on failure.
func mustRunFullEnv(ctx context.Context, dir string, fullEnv []string, name string, args ...string) (runResult, error) {
	return checkRun(runFullEnv(ctx, dir, fullEnv, name, args...))
}

func checkRun(res runResult) (runResult, error) {
	if res.Err != nil {
		return res, fmt.Errorf("command failed (exit %d): %s\n--- stderr ---\n%s\n--- stdout ---\n%s",
			res.ExitCode, res.Cmd, tail(res.Stderr, 4000), tail(res.Stdout, 2000))
	}
	return res, nil
}

// envWithout returns a copy of environ with the named variables removed
// (matched on the "NAME=" prefix, case-sensitive).
func envWithout(environ []string, names ...string) []string {
	drop := make(map[string]bool, len(names))
	for _, n := range names {
		drop[n] = true
	}
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		if drop[key] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// tail returns the last n characters of s (for keeping logs bounded).
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…(truncated)…" + s[len(s)-n:]
}

// commandExists reports whether a binary is resolvable on PATH.
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
