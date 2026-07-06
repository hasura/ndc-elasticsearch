//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ddnLoginOnce ensures we authenticate the ddn CLI at most once per test process.
var (
	ddnLoginOnce sync.Once
	ddnLoginErr  error
)

// ensureDDNLogin authenticates the ddn CLI. This CLI version (v3.9.x, installed
// via the v4 channel) requires an authenticated session even for local-only
// `supergraph init` / `supergraph build local` — it fails otherwise with
// "Not logged in. Run `ddn auth login`". When HASURA_DDN_PAT is set (a CI
// secret, or a local dev's PAT) we log in non-interactively. If it is empty we
// assume the caller already ran `ddn auth login` (e.g. via browser) and let the
// subsequent commands surface a clear error if not.
//
// Note: `--access-token` is the current flag (the older `--pat` is deprecated);
// both accept a Personal Access Token. Login persists credentials to the ddn
// config, so a single login covers every case in the run.
func ensureDDNLogin(ctx context.Context, dir string, baseEnv []string) error {
	ddnLoginOnce.Do(func() {
		pat := strings.TrimSpace(os.Getenv("HASURA_DDN_PAT"))
		if pat == "" {
			return // assume already authenticated (or a subsequent step will fail clearly)
		}
		if _, err := mustRun(ctx, dir, baseEnv, "ddn", "auth", "login",
			"--access-token", pat, "--no-prompt"); err != nil {
			ddnLoginErr = fmt.Errorf("ddn auth login (using HASURA_DDN_PAT): %w", err)
		}
	})
	return ddnLoginErr
}

// connectorLinkName is the DataConnectorLink / connector name used in the
// generated DDN metadata. It matches the connector's docker-compose service
// name (`ndc-elasticsearch`) is reached at build time via the env rewrite below.
const connectorLinkName = "elasticsearch"

// dockerConnectorURL is the URL the engine (running inside the compose network)
// uses to reach the connector. Must match the compose service name + port.
const dockerConnectorURL = "http://ndc-elasticsearch:8080"

// BuildDDN drives the REAL `ddn` CLI to generate the engine metadata from the
// running connector, following decision #2 of the spec:
//
//	ddn supergraph init            (scaffold a fresh, version-matched project)
//	ddn connector-link add         (register a link to the running connector)
//	ddn connector-link update      (introspect the connector /schema)
//	ddn model add <link> "*"       (auto-create a model per collection/index)
//	ddn supergraph build local     (assemble engine metadata)
//
// Metadata is NEVER hand-edited, so any index that ES exposes automatically
// becomes a GraphQL model. The only mechanical transform is swapping the
// connector host in the generated env from the host-visible localhost:<port>
// (needed for introspection from the test process) to the docker-network URL
// (needed by the engine container at runtime) — see the env rewrite below.
//
// The exact `ddn` sub-command flags are the surface most likely to need
// adjustment across CLI versions; the pinned/supported version is documented in
// e2e/README.md.
func BuildDDN(ctx context.Context, s *Stack, c Case) error {
	t := s.startTimer("ddn:build")
	defer t.done()

	if !commandExists("ddn") {
		return fmt.Errorf("`ddn` CLI not found on PATH; install the Hasura DDN CLI (see e2e/README.md)")
	}

	projDir := filepath.Join(s.Workdir, "ddn")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		return err
	}

	hostConnectorURL := fmt.Sprintf("http://localhost:%d", s.Ports["connector"])
	// ddn reads/writes non-interactively when these are set.
	baseEnv := []string{
		"HASURA_DDN_PAT=" + os.Getenv("HASURA_DDN_PAT"), // optional; local builds don't require login
	}

	// 0. Authenticate the ddn CLI (required even for local-only builds).
	if err := ensureDDNLogin(ctx, projDir, baseEnv); err != nil {
		return err
	}

	// 1. Scaffold a fresh supergraph project (globals + app subgraphs).
	if _, err := mustRun(ctx, projDir, baseEnv, "ddn", "supergraph", "init", ".", "--no-prompt"); err != nil {
		return fmt.Errorf("ddn supergraph init: %w", err)
	}

	// ddn ≥ v3.8 uses the project-root .env as the context's localEnvFile and
	// requires the target env file to exist before connector-link add can write
	// connector URLs into it. supergraph init creates .env (empty) — we use it
	// so that supergraph build local (which reads localEnvFile=".env") sees the URL.
	localEnvFile := filepath.Join(projDir, ".env")

	// 2. Register a connector link pointing at the running connector (host URL
	//    so the following introspection call can reach it from the test host).
	if _, err := mustRun(ctx, projDir, baseEnv, "ddn", "connector-link", "add", connectorLinkName,
		"--configure-host", hostConnectorURL,
		"--subgraph", filepath.Join("app", "subgraph.yaml"),
		"--target-env-file", localEnvFile,
	); err != nil {
		return fmt.Errorf("ddn connector-link add: %w", err)
	}

	// 3. Introspect the connector schema into the DataConnectorLink.
	if _, err := mustRun(ctx, projDir, baseEnv, "ddn", "connector-link", "update", connectorLinkName,
		"--subgraph", filepath.Join("app", "subgraph.yaml"),
		"--env-file", localEnvFile,
	); err != nil {
		return fmt.Errorf("ddn connector-link update (introspection): %w", err)
	}

	// Rewrite the connector host in the env from localhost:<port> (host) to the
	// docker-network URL so the engine container can reach the connector.
	if err := rewriteConnectorHost(localEnvFile, hostConnectorURL, dockerConnectorURL); err != nil {
		return fmt.Errorf("rewriting connector host for docker network: %w", err)
	}

	// 4. Add a model for every collection the connector exposes.
	if _, err := mustRun(ctx, projDir, baseEnv, "ddn", "model", "add", connectorLinkName, "*",
		"--subgraph", filepath.Join("app", "subgraph.yaml"),
	); err != nil {
		return fmt.Errorf("ddn model add: %w", err)
	}
	// Commands/relationships are best-effort (a pure-model connector may have none).
	_ = run(ctx, projDir, baseEnv, "ddn", "command", "add", connectorLinkName, "*",
		"--subgraph", filepath.Join("app", "subgraph.yaml"))

	// 5. Build the engine metadata locally.
	buildOut := filepath.Join(projDir, "engine-build")
	if _, err := mustRun(ctx, projDir, baseEnv, "ddn", "supergraph", "build", "local",
		"--output-dir", buildOut,
	); err != nil {
		return fmt.Errorf("ddn supergraph build local: %w", err)
	}

	// Strip AuthConfig from the built metadata — ddn supergraph build embeds it
	// (required by the CLI) but v3-engine:latest reads auth config only from
	// AUTHN_CONFIG_PATH and rejects AuthConfig as an unknown kind in the metadata.
	metaPath, err := findBuiltMetadata(buildOut)
	if err != nil {
		return fmt.Errorf("locating built metadata: %w", err)
	}
	if err := stripUnknownKindsFromMetadata(metaPath); err != nil {
		return fmt.Errorf("stripping unknown kinds from metadata: %w", err)
	}

	// Stage the artifacts the engine container mounts: the ddn-built open-dd
	// metadata + the repo's webhook auth config (matches the repo compose).
	if err := stageEngineArtifacts(s, buildOut); err != nil {
		return err
	}
	return nil
}

// rewriteConnectorHost replaces every occurrence of oldURL with newURL in the
// given env file (used to point the engine at the docker-network connector URL
// after host-side introspection).
func rewriteConnectorHost(envFile, oldURL, newURL string) error {
	b, err := os.ReadFile(envFile)
	if err != nil {
		return err
	}
	updated := strings.ReplaceAll(string(b), oldURL, newURL)
	// Also handle the bare host:port form in case ddn stored it without scheme.
	oldHostPort := strings.TrimPrefix(oldURL, "http://")
	newHostPort := strings.TrimPrefix(newURL, "http://")
	updated = strings.ReplaceAll(updated, oldHostPort, newHostPort)
	return os.WriteFile(envFile, []byte(updated), 0o644)
}

// stageEngineArtifacts locates the built metadata and copies it (plus the
// webhook auth config) into ${workdir}/engine which the engine container mounts.
func stageEngineArtifacts(s *Stack, buildOut string) error {
	metaSrc, err := findBuiltMetadata(buildOut)
	if err != nil {
		return err
	}
	engineDir := filepath.Join(s.Workdir, "engine")
	if err := copyFile(metaSrc, filepath.Join(engineDir, "metadata.json")); err != nil {
		return err
	}
	// The OSS v3-engine reads a separate AUTHN_CONFIG_PATH.
	// v3-dev-auth-webhook listens on port 3060 (not 3050); write the config
	// directly so we don't depend on the repo's resources/auth_config.json port.
	authConfig := `{
    "version": "v1",
    "definition": {
        "allowRoleEmulationBy": "admin",
        "mode": {
            "webhook": {
                "url": "http://auth_hook:3060/validate-request",
                "method": "Post"
            }
        }
    }
}`
	authDst := filepath.Join(engineDir, "auth_config.json")
	if err := os.MkdirAll(engineDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(authDst, []byte(authConfig), 0o644); err != nil {
		return fmt.Errorf("staging auth_config.json: %w", err)
	}
	return nil
}

// findBuiltMetadata locates metadata.json under the build output directory
// (ddn versions have placed it at <out>/metadata.json or <out>/metadata.json
// under an engine/ subdir).
func findBuiltMetadata(buildOut string) (string, error) {
	candidates := []string{
		filepath.Join(buildOut, "metadata.json"),
		filepath.Join(buildOut, "engine", "metadata.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	// Fall back to a recursive search for the largest metadata.json.
	var found string
	_ = filepath.Walk(buildOut, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(path) == "metadata.json" {
			found = path
		}
		return nil
	})
	if found != "" {
		return found, nil
	}
	return "", fmt.Errorf("could not locate built metadata.json under %s", buildOut)
}

// engineKnownKinds is the set of metadata object kinds that v3-engine:latest
// accepts. Kinds produced by ddn supergraph build but absent here (e.g.
// AuthConfig, CompatibilityConfig) are stripped before staging the metadata.
var engineKnownKinds = map[string]bool{
	"DataConnectorLink":                 true,
	"GraphqlConfig":                     true,
	"ObjectType":                        true,
	"ScalarType":                        true,
	"ObjectBooleanExpressionType":       true,
	"BooleanExpressionType":             true,
	"OrderByExpression":                 true,
	"DataConnectorScalarRepresentation": true,
	"AggregateExpression":               true,
	"Model":                             true,
	"Command":                           true,
	"Relationship":                      true,
	"TypePermissions":                   true,
	"ModelPermissions":                  true,
	"CommandPermissions":                true,
	"LifecyclePluginHook":               true,
	"View":                              true,
	"ViewPermissions":                   true,
}

// stripUnknownKindsFromMetadata removes metadata objects whose kind is not
// recognised by v3-engine:latest. ddn supergraph build embeds kinds like
// AuthConfig and CompatibilityConfig that the engine reads from separate
// config files (AUTHN_CONFIG_PATH) or ignores entirely.
func stripUnknownKindsFromMetadata(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var meta map[string]interface{}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	subgraphs, _ := meta["subgraphs"].([]interface{})
	for _, sg := range subgraphs {
		sgMap, ok := sg.(map[string]interface{})
		if !ok {
			continue
		}
		objects, _ := sgMap["objects"].([]interface{})
		filtered := make([]interface{}, 0, len(objects))
		for _, obj := range objects {
			objMap, ok := obj.(map[string]interface{})
			if ok && !engineKnownKinds[objMap["kind"].(string)] {
				continue
			}
			filtered = append(filtered, obj)
		}
		sgMap["objects"] = filtered
	}
	out, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
