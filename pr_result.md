# PR result — fix ES 401-retry empty-body / dropped-payload bug

## Branch & PR

- **Branch:** `fix/es-401-retry-empty-body`
- **PR (DRAFT):** https://github.com/hasura/ndc-elasticsearch/pull/118
- **Base:** `main` (at `8a3eb41`, the v1.10.2 changelog commit)
- **Assignee:** requested `--assignee m-Bilal`. ⚠️ The assignment could **not** be applied
  by the authenticated account (`dliub`) — GitHub returned `403 Must have admin rights to
  Repository` / missing `read:project` scope. A maintainer with write/admin access should
  add **@m-Bilal** as assignee (one click in the PR sidebar, or
  `gh pr edit 118 --add-assignee m-Bilal`).
- **Commit trailer:** `Co-authored-by: Mohd Bilal <bilal@hasura.io>` is present on the commit.
- Links in PR description: PromptQL thread
  `https://prompt.ql.app/project/hasuraql/promptql-playground/thread/ff214ba1-1075-444d-a3ba-56f11ed6c3d8`
  and **JPMorgan Chase support ticket #15000**.

## What changed

- `elasticsearch/client.go` — `search()`:
  1. **Fix (primary):** the encoded request body is captured once and a fresh
     `bytes.Reader` is rebuilt **before every attempt**, so the post-401 retry sends the
     identical, correct `_search` body instead of the drained/empty buffer.
  2. **Fix (secondary):** the transport error from the first `req.Do` is checked **before**
     `res.IsError()`, guarding the nil-pointer panic on a transport-level failure.
  3. **Per-attempt request logging** via the existing `connector.GetLogger(ctx)` (`*slog.Logger`,
     `DebugContext`): logs the actual `body` + `index` sent on each attempt. First attempt is
     `msg="Query"`; the retry is `msg="Retry Query"` (literal **"Retry Query"** marker). Bodies
     reflect what is truly sent (empty under the old bug). No secrets/auth headers logged.
- `elasticsearch/client_retry_test.go` — hermetic functional tests (httptest fake ES).

## CI / local checks

| Check | Command | Result |
|-------|---------|--------|
| Build | `go build ./...` | ✅ pass |
| Vet   | `go vet ./...` | ✅ pass |
| Format (changed files) | `gofmt -l elasticsearch/client.go elasticsearch/client_retry_test.go` | ✅ clean |
| Unit tests (race) | `make unit-test` → `go test -v -race -timeout 3m ./...` | ✅ all packages pass |
| Lint | `golangci-lint run` | ⚠️ not installed locally; GitHub `Test` workflow runs `go build` + `make unit-test` only (no golangci in CI). SonarCloud ignored per instructions. |

> Note: `gofmt -l .` also flags some **pre-existing** unformatted files
> (`connector/connector.go`, `connector/fields.go`, `connector/filter.go`,
> `connector/query_test.go`, `internal/fields_test.go`) that exist on the base branch and are
> unrelated to this change — intentionally left untouched.

## How to run the functional test — OLD (buggy) vs NEW (fixed)

The switch is the env var **`ELASTICSEARCH_DISABLE_RETRY_BODY_REBUILD`**. When set, the
production code skips the body rebuild before the retry, exercising the **real** code path in
its pre-fix (buggy) form. When unset, the fix is active.

```bash
cd ndc-elasticsearch

# NEW (fixed) — retry body equals the first attempt body, non-empty. Test PASSES:
go test -v -run TestSearchRetryBodyOn401 ./elasticsearch/

# OLD (buggy) — retry sends an EMPTY body (ES would treat it as match_all). Test FAILS:
ELASTICSEARCH_DISABLE_RETRY_BODY_REBUILD=1 go test -v -run TestSearchRetryBodyOn401 ./elasticsearch/
```

Observed:

```
# fixed
--- PASS: TestSearchRetryBodyOn401
    OK: both attempts sent identical, non-empty query body: {"query":{"term":{"customer_id":"JPMC-42"}},"size":10}

# buggy
--- FAIL: TestSearchRetryBodyOn401
    BUG REPRODUCED: retry sent an EMPTY body (Elasticsearch would treat this as match_all
    and return unfiltered results). retry body=""
```

## How to view the per-attempt logs (incl. the "Retry Query"-marked retry line)

A dedicated test captures the logs and prints them with `-v`:

```bash
# Fixed mode — both attempts log the full body; retry carries the "Retry Query" marker:
go test -v -run TestSearchPerAttemptLogging ./elasticsearch/

# Buggy mode — the "Retry Query" line shows an EMPTY body (matches what's actually sent):
ELASTICSEARCH_DISABLE_RETRY_BODY_REBUILD=1 go test -v -run TestSearchPerAttemptLogging ./elasticsearch/
```

Sample captured log lines (fixed mode):

```json
{"level":"DEBUG","msg":"Query","index":"transactions","body":"{\"query\":{\"term\":{\"customer_id\":\"JPMC-42\"}}}\n"}
{"level":"DEBUG","msg":"Retry Query","index":"transactions","body":"{\"query\":{\"term\":{\"customer_id\":\"JPMC-42\"}}}\n"}
```

Buggy mode prints the same `Retry Query` line but with `"body":""`.

### In a running connector

The logging uses the connector's existing logger at **DEBUG** level. Run the connector with
debug logging enabled and grep for the marker:

```bash
HASURA_LOG_LEVEL=debug <run the connector> 2>&1 | grep "Retry Query"
```

Each `_search` over the wire emits a `Query` line on the first attempt; any post-401 retry
emits a `Retry Query` line — both include the `index` and the actual request `body`.
