# AGENTS.md

Guidance for AI coding agents working in this repository. Human-facing docs live in [README.md](README.md) and [docs/](docs/).

## What this is

`ndc-elasticsearch` is a Hasura DDN **data connector** for Elasticsearch, written in Go. It implements the NDC spec **v0.2** using `github.com/hasura/ndc-sdk-go/v2` (v2.3.2). It's read-only: queries, aggregates, explain, and native queries are supported, and mutations return `NotSupportedError`.

Flow: an NDC `QueryRequest` is translated to Elasticsearch DSL, sent to `_search`, and the hits/aggregations are mapped back into NDC `RowSet`s. See [docs/architecture.md](docs/architecture.md).

## Layout

| Path | Purpose |
|---|---|
| [main.go](main.go) | Entry point: `connector.StartCustom[types.Configuration, types.State]` |
| [cli/](cli/) | Extra CLI subcommands: `initialize`, `update` (introspects ES and writes `configuration.json`), `validate`, `upgrade`, `version`. `serve` comes from the SDK. |
| [connector/](connector/) | NDC implementation. `connector.go` handles state init and capabilities, `schema.go` turns the config into an NDC schema, `query.go` is the query pipeline, and `filter.go`, `sort.go`, `select.go`, `aggregate.go`, `variables.go`, `native_query.go` and `response.go` each build or parse one part of the DSL |
| [elasticsearch/](elasticsearch/) | ES client wrapper: env/credentials-provider auth, TLS/CA, reauth and retry on 401, search, mappings and aliases |
| [internal/](internal/) | Static scalar/object type tables (`static_types.go`), field-type helpers, misc utils |
| [types/](types/) | `Configuration`, `State`, `NativeQuery`, `PostProcessor` |
| [testdata/](testdata/), [connector/testdata/](connector/testdata/) | Unit test fixtures |
| [e2e/](e2e/) | Real ES + connector + DDN end-to-end suite (build tag `e2e`) |
| [ci/templates/](ci/templates/) | DDN connector packaging metadata. Supported env vars are declared here. |
| [resources/](resources/), [docker-compose.yaml](docker-compose.yaml) | Local dev stack (ES, Kibana, v3-engine, Jaeger, Prometheus) and sample data |

## Commands

```sh
go build ./...            # build
make unit-test            # go test -v -race -timeout 3m ./...  (what CI runs on PRs)
make format               # gofmt -w -s .
make lint                 # golangci-lint run (default config; no .golangci file)
go vet ./...

# e2e (needs Docker, the `ddn` CLI, and HASURA_DDN_PAT or a prior `ddn auth login`)
make e2e                              # all cases
make e2e-case CASE=custom_products    # one case
make -C e2e e2e-update-golden CASE=<name>   # regenerate goldens, then review and commit them
```

`make generate` points to `scripts/generate.sh`, which doesn't exist. Don't rely on it.

Before you finish a change, run `gofmt`, `go vet ./...` and `make unit-test`. Unit tests don't need a running Elasticsearch.

## Runtime configuration

- Config directory: `HASURA_CONFIGURATION_DIRECTORY` (Docker default `/etc/connector`). The file in it is `configuration.json`, which has `indices` (raw ES mappings keyed by index or alias name) and `queries` (native queries).
- The CLI reads its directory from `--configuration` or `HASURA_PLUGIN_CONNECTOR_CONTEXT_PATH`.
- ES connection env vars: `ELASTICSEARCH_URL`, `ELASTICSEARCH_USERNAME`/`ELASTICSEARCH_PASSWORD` or `ELASTICSEARCH_API_KEY`, `ELASTICSEARCH_CA_CERT_PATH`, `ELASTICSEARCH_INDEX_PATTERN`, and `ELASTICSEARCH_DEFAULT_RESULT_SIZE` (default 10000). There's also a credentials-provider mode: `HASURA_CREDENTIALS_PROVIDER_URI`, `ELASTICSEARCH_CREDENTIALS_PROVIDER_KEY`, `ELASTICSEARCH_CREDENTIALS_PROVIDER_MECHANISM` = `api-key` | `service-token` | `bearer-token`.
- If you add a user-facing env var, also add it to [ci/templates/connector-metadata.yaml](ci/templates/connector-metadata.yaml) and the docs.

## Invariants and gotchas

**Schema generation ([connector/schema.go](connector/schema.go))**
- Nested object types are always named with the full path, `<index>.<path.to.field>` (for example `products.manufacturer`). Never collapse them to bare names, even when the name is unique. Bare names caused fields to be dropped silently when names collided (ENT-82, #123), and they let a newly added index rename existing GraphQL types. Tests: [schema_collision_test.go](connector/schema_collision_test.go).
- `/schema` output must be byte-for-byte deterministic for the same config. Go map iteration is random, so sort any slice you build from map iteration (`Collections`, `Functions`, `Procedures` are sorted by name). Tests: [schema_ordering_test.go](connector/schema_ordering_test.go). Changing the schema output also changes the e2e `golden.schema.json` files and is usually a breaking change for users.
- `ParseConfigurationToSchema` has side effects: it fills the per-index lookup maps on `State` (`SupportedSortFields`, `SupportedAggregateFields`, `SupportedFilterFields`, `NestedFields`) that the query builders depend on. Any test that builds queries has to call it first, as [query_test.go](connector/query_test.go) does.

**Configuration contract**
- The native query return-type kind is spelled `"defination"` (sic) in user configs. It's part of the public config format, so don't "fix" the spelling.
- Aliases from `update` are written as separate entries in `indices`, with mappings copied from their target index.

**Query pipeline**
- `PostProcessor` is passed through `context.WithValue(ctx, "postProcessor", ...)` with a string key, and the default result size uses `elasticsearch.DEFAULT_RESULT_SIZE_KEY`. Follow the existing pattern; any new entry point has to set the postProcessor.
- Text fields can have keyword sub-fields. Sort, aggregation and term-level filters pick the best sub-field through `internal.GetBestFieldOrSubFieldFor*`. Check that path before you add an operator.
- The 401 retry in `elasticsearch/client.go` has to resend the full request body (an earlier bug sent an empty body, which returned unfiltered results). Don't log query bodies or filter values, since they may contain PII; log payload sizes instead.

**Tests**
- Query builder tests are fixture-driven. [query_test.go](connector/query_test.go) lists `{group, name}` pairs and reads `testdata/unit_tests/query_tests/<group>/configuration.json` and `<group>/<name>/{ndc_request.json,want.json}`. To add a case, add the files and a new entry to the `tests` slice.
- Filter predicate tests use `connector/testdata/filter_tests/get_predicate/<case>/{got,want}.json`. Some tests have a commented-out `os.WriteFile` that regenerates goldens. Only uncomment it temporarily.
- [connector/connector_test.go](connector/connector_test.go) is commented out entirely because it was broken. Leave it alone unless the task is to fix it.
- Assertions use `testify/assert`. Compare JSON with `assert.JSONEq`.
- E2E cases are file-drop: you add a directory under `e2e/cases/<name>/` and never touch `e2e/harness/`. See [e2e/README.md](e2e/README.md).

## Conventions

- Match the surrounding style: `map[string]interface{}` for ES documents and DSL, errors returned up the stack, and NDC errors built with `schema.UnprocessableContentError` / `schema.NotSupportedError`.
- Tracing: create a child span per stage with `state.Tracer.Start(ctx, "<snake_case_name>")` and record failures with `span.SetStatus(codes.Error, ...)`.
- Add user-visible changes under `## [Unreleased]` in [CHANGELOG.md](CHANGELOG.md) (Keep a Changelog format; mark breaking changes with **BREAKING:**).
- The Go version is pinned in `go.mod` and the `Dockerfile` (`golang:1.26.10`, `GOTOOLCHAIN`). Bump both together.
- Pin GitHub Actions to commit SHAs, as the existing workflows do.
