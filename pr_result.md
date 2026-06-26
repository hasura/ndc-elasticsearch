# PR result — fix ES 401-retry empty-body / dropped-payload bug

## Branch & PR

- **Branch:** `fix/es-401-retry-empty-body`
- **PR (DRAFT):** https://github.com/hasura/ndc-elasticsearch/pull/118
- **Base:** `main` (at `8a3eb41`, the v1.10.2 changelog commit)
- **Assignee:** requested `--assignee m-Bilal`. ⚠️ Could **not** be applied by the authenticated
  account (`dliub`) — GitHub returned `403 Must have admin rights to Repository` / missing
  `read:project` scope. A maintainer with write/admin access should add **@m-Bilal** as
  assignee (`gh pr edit 118 --add-assignee m-Bilal`).
- **Commit trailer:** `Co-authored-by: Mohd Bilal <bilal@hasura.io>` is present on the commits.
- Links in PR description: PromptQL thread
  `https://prompt.ql.app/project/hasuraql/promptql-playground/thread/ff214ba1-1075-444d-a3ba-56f11ed6c3d8`
  and **JPMorgan Chase support ticket #15000**.

## Reviewer follow-up applied: testing-only env-var toggle REMOVED

Per Mohd Bilal's review, the `ELASTICSEARCH_DISABLE_RETRY_BODY_REBUILD` env-var toggle has been
**removed entirely**:

- Deleted the `retryBodyRebuildDisabled()` helper and the `if !retryBodyRebuildDisabled() { ... }`
  guard. The retry now **always** rebuilds the body: `req.Body = bytes.NewReader(body)`
  unconditionally before the retry `req.Do`.
- Deleted the `retryBodyForLog()` peek helper; the retry log now logs `string(body)`, which is
  exactly the bytes the rebuilt reader sends over the wire (no divergence between logged and
  sent body).
- The tests no longer reference the env var. There is **one** functional test command (fixed
  mode only). To exercise the old bug, a developer comments out the rebuild line in `search()`
  (see below).
- `os` import is retained in `client.go` because `GetIndices` still uses `os.Getenv`.

## What the change does (unchanged intent)

- `elasticsearch/client.go` — `search()`:
  1. **Fix (primary):** the encoded request body is captured once and a fresh `bytes.Reader` is
     rebuilt **before every attempt**, so the post-401 retry sends the identical, correct
     `_search` body instead of the drained/empty buffer (ES treats an empty `_search` body as
     `match_all` ⇒ unfiltered results).
  2. **Fix (secondary):** the transport error from the first `req.Do` is checked **before**
     `res.IsError()`, guarding the nil-pointer panic on a transport-level failure.
  3. **Per-attempt request logging** via the existing `connector.GetLogger(ctx)` (`*slog.Logger`,
     `DebugContext`): logs the actual `body` + `index` sent on each attempt. First attempt is
     `msg="Query"`; the retry is `msg="Retry Query"` (literal **"Retry Query"** marker). No
     secrets/auth headers logged.
- `elasticsearch/client_retry_test.go` — hermetic functional tests (httptest fake ES, no real
  cluster).

## How to run the functional test (single fixed-mode command, no env var)

```bash
cd ndc-elasticsearch
go test -v -run TestSearchRetryBodyOn401 ./elasticsearch/
```

Observed output:

```
--- PASS: TestSearchRetryBodyOn401
    OK: both attempts sent identical, non-empty query body: {"query":{"term":{"customer_id":"JPMC-42"}},"size":10}
```

## How to reproduce the OLD buggy behavior manually (no env var)

In `elasticsearch/client.go`, inside `search()`, comment out the body-rebuild line before the
retry `req.Do`:

```go
            // req.Body = bytes.NewReader(body)   // <- comment this out
            logger.DebugContext(ctx, "Retry Query", "index", index, "body", string(body))
            res, err = req.Do(ctx, e.client)
```

Then re-run the same test — it fails, reproducing the bug:

```
--- FAIL: TestSearchRetryBodyOn401
    BUG REPRODUCED: retry sent an EMPTY body (Elasticsearch would treat this as match_all
    and return unfiltered results). retry body=""
```

(Restore the line afterwards.) Note: with the line commented out the logged retry body
(`string(body)`) would no longer match the wire body — which is exactly why the rebuild must
stay; the test guards against the empty wire body.

## How to view the per-attempt logs (incl. the "Retry Query"-marked retry line)

```bash
go test -v -run TestSearchPerAttemptLogging ./elasticsearch/
```

Sample captured log lines:

```json
{"level":"DEBUG","msg":"Query","index":"transactions","body":"{\"query\":{\"term\":{\"customer_id\":\"JPMC-42\"}}}\n"}
{"level":"DEBUG","msg":"Retry Query","index":"transactions","body":"{\"query\":{\"term\":{\"customer_id\":\"JPMC-42\"}}}\n"}
```

Observed test result:

```
--- PASS: TestSearchPerAttemptLogging
    retry logged the full body: {"query":{"term":{"customer_id":"JPMC-42"}}}
```

### In a running connector

The logging uses the connector's existing logger at **DEBUG** level. Run with debug logging and
grep for the marker:

```bash
HASURA_LOG_LEVEL=debug <run the connector> 2>&1 | grep "Retry Query"
```

Each `_search` over the wire emits a `Query` line on the first attempt; any post-401 retry emits
a `Retry Query` line — both include the `index` and the actual request `body`.

## CI / local checks (actual observed output)

- `go build ./...` → **OK** (no output)
- `go vet ./...` → **OK** (no output)
- `gofmt -l elasticsearch/client.go elasticsearch/client_retry_test.go` → **clean** (no output)
- `make unit-test` (`go test -v -race -timeout 3m ./...`):

```
?   	github.com/hasura/ndc-elasticsearch	[no test files]
?   	github.com/hasura/ndc-elasticsearch/cli	[no test files]
ok  	github.com/hasura/ndc-elasticsearch/connector	1.644s
ok  	github.com/hasura/ndc-elasticsearch/elasticsearch	2.100s
ok  	github.com/hasura/ndc-elasticsearch/internal	2.278s
ok  	github.com/hasura/ndc-elasticsearch/types	1.806s
```

- `golangci-lint run` → not installed locally; the GitHub `Test` workflow runs only `go build` +
  `make unit-test` (no golangci in CI). SonarCloud ignored per instructions.

> Note: `gofmt -l .` also flags pre-existing unformatted files (`connector/connector.go`,
> `connector/fields.go`, `connector/filter.go`, `connector/query_test.go`,
> `internal/fields_test.go`) that exist on the base branch and are unrelated to this change —
> intentionally left untouched.
