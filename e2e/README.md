# ndc-elasticsearch end-to-end (e2e) test suite

A **true** end-to-end suite: it spins up a real Elasticsearch in Docker, runs the
real `ndc-elasticsearch` connector against it, stands up a real Hasura DDN
v3-engine via the real `ddn` CLI, and then asserts that

1. the connector's introspected schema matches the Elasticsearch schema (**L3**), and
2. GraphQL queries sent to DDN return the **same result set** as the equivalent
   query sent directly to Elasticsearch (**L4**).

No mocks, no stubs — every component is the real thing.

Adding a new test case is a **file-drop**: you never touch the harness or the
setup. Drop a directory under [`cases/`](cases/) with a schema, some data, and a
few queries, and the runner discovers and executes it automatically.

---

## Table of contents

- [How it works](#how-it-works)
- [Layout](#layout)
- [Prerequisites](#prerequisites)
- [Running locally](#running-locally)
- [Running in CI](#running-in-ci)
- [The report](#the-report)
- [**How to add a test case in 3 steps**](#how-to-add-a-test-case-in-3-steps)
- [Case file reference](#case-file-reference)
- [Assertions in detail](#assertions-in-detail)
- [Environment variables](#environment-variables)
- [Known quirks](#known-quirks)
- [First-run checklist for maintainers](#first-run-checklist-for-maintainers)

---

## How it works

Each case gets a **fresh, fully isolated stack** (its own docker compose project,
named `e2e-<case>`), which is seeded only from that case's files, asserted, and
then torn down. This keeps every tricky behaviour a small, isolated,
easily-debuggable case.

Per case, the harness ([`harness/`](harness/)) runs these stages:

1. **Elasticsearch up** — cert bootstrap + `es01` (TLS on), waits for auth.
2. **Seed** — apply the case's `indices/`, `init/`, `data/`, and optional Kibana
   sample dataset into the fresh ES.
3. **Introspect** — run the connector's real `ndc-elasticsearch update` (a
   one-shot container) to generate `configuration.json` from the live ES.
4. **Connector up** — start the connector (`serve`) with that configuration.
5. **Schema golden** — snapshot the connector's full `GET /schema` and compare it
   against the committed `golden.schema.json` (see [below](#assertions-in-detail)).
6. **L3 assertions** — schema conformance (see [below](#assertions-in-detail)).
7. **DDN build** — drive the real `ddn` CLI:
   `supergraph init` → `connector-link add` → `connector-link update`
   (introspect) → `model add "*"` → `supergraph build local`. Metadata is
   **never hand-edited**, so every index automatically becomes a GraphQL model.
8. **Engine up** — start the DDN v3-engine + dev auth webhook on the built metadata.
9. **L4 assertions** — for each query: POST GraphQL to the engine and the
   equivalent DSL to ES, then compare both results against their committed golden
   files (see [below](#assertions-in-detail)).

Everything is gated behind the `e2e` build tag **and** `E2E=1`, so the existing
fast unit CI (`make unit-test`) is completely unaffected.

## Layout

```
e2e/
  Makefile                    targets: e2e, e2e-case, e2e-update-golden, e2e-report, e2e-clean
  docker-compose.e2e.yaml     per-case stack template (pinned to STACK_VERSION, default 8.13.4)
  harness/                    the ONLY code — never edited to add a case
    e2e_test.go               discovery + per-case orchestration + L3/L4 + report
    config.go  discover.go  stack.go  seed.go  es.go  ddn.go
    schema_assert.go  schema_golden.go  typemap.go  graphql.go  report.go  exec.go  doc.go
  cases/
    kibana_sample_logs/       reference case using the elastic.co "logs" sample dataset
    custom_products/          fully-custom example (indices/ + data/ + init/ + queries/)
  report/                     e2e-report.json + e2e-report.md (generated each run)
```

## Prerequisites

- **Docker** + the `docker compose` plugin.
- **Go** (version from [`go.mod`](../go.mod)).
- **The Hasura DDN CLI (`ddn`)** on your `PATH`.
  Install: `curl -L https://graphql-engine-cdn.hasura.io/ddn/cli/v4/get.sh | bash`
  (the CI workflow installs the v4 channel; verified against `ddn` v3.9.x — the
  harness only uses the stable `supergraph init` / `connector-link` /
  `model add` / `supergraph build local` sub-commands — if your CLI version
  renames a flag, adjust [`harness/ddn.go`](harness/ddn.go)).
- **DDN authentication.** This `ddn` version requires an authenticated session
  even for local-only builds. Set `HASURA_DDN_PAT` to a Hasura DDN Personal
  Access Token and the harness logs in automatically
  (`ddn auth login --access-token "$HASURA_DDN_PAT"`) before any `ddn` command.
  Alternatively, if `HASURA_DDN_PAT` is unset, the harness assumes you have
  already run `ddn auth login` interactively (e.g. via browser).

## Running locally

```bash
export HASURA_DDN_PAT=...   # or run `ddn auth login` interactively beforehand

# run everything
make -C e2e e2e

# run a single case (fast iteration)
make -C e2e e2e-case CASE=custom_products

# (re)generate goldens for a case, then commit them
make -C e2e e2e-update-golden CASE=custom_products

# leave the stack up to poke at it after a failure
KEEP_STACK=1 make -C e2e e2e-case CASE=custom_products

# view the last report
make -C e2e e2e-report
```

Fail-fast is **off** locally by default. Turn it on with `FAIL_FAST=1`.

## Running in CI

CI runs on **every PR**, in a **separate** workflow (`.github/workflows/e2e.yaml`)
from the unit tests so the unit CI stays fast. It sets `FAIL_FAST=1` (fail-fast
**on** in CI) and always uploads the report as an artifact.

The workflow requires a `HASURA_DDN_PAT` repository secret (a Hasura DDN Personal
Access Token). The harness uses it to run `ddn auth login` non-interactively before
any `ddn` command. If the CLI already has a valid session when the token is present,
the login is skipped automatically.

## The report

Every run always produces (even on failure):

- `e2e/report/e2e-report.json` — machine-readable, per case → query: the layer
  (L3/L4), pass/fail, the raw DDN/ES/golden payloads on failure, and per-step
  timings.
- `e2e/report/e2e-report.md` — the same, human-readable, with collapsible
  payload blocks. In CI it is also written to the job summary.

## How to add a test case in 3 steps

> You only ever add **files**. You never edit the harness or the compose file.

**Step 1 — create the case directory and its schema + data.**

```bash
mkdir -p e2e/cases/my_case/{indices,data,queries}

# the ES mapping IS the schema; the file base name is the index name
cat > e2e/cases/my_case/indices/orders.mapping.json <<'JSON'
{ "mappings": { "properties": {
  "id":     { "type": "keyword" },
  "total":  { "type": "double" },
  "status": { "type": "keyword" }
} } }
JSON

# bulk NDJSON (action line + document line, repeated)
cat > e2e/cases/my_case/data/orders.ndjson <<'JSON'
{"index":{"_index":"orders","_id":"1"}}
{"id":"1","total":10.5,"status":"paid"}
{"index":{"_index":"orders","_id":"2"}}
{"id":"2","total":3.25,"status":"pending"}
JSON
```

**Step 2 — add one query directory per query you want to test.**

```bash
mkdir -p e2e/cases/my_case/queries/paid_orders

# the GraphQL sent to DDN
# Use ES-native operators: term, match, range, _and/_or/_not, etc.
# The model name matches the index name (camelCase if multi-word).
# order_by direction is Asc or Desc (capitalised).
cat > e2e/cases/my_case/queries/paid_orders/query.graphql <<'GQL'
query PaidOrders {
  orders(where: { status: { term: "paid" } }, order_by: { id: Asc }) {
    id
    total
  }
}
GQL

# the equivalent raw ES DSL (sent to /<index>/_search)
cat > e2e/cases/my_case/queries/paid_orders/es_search.json <<'JSON'
{ "_source": ["id","total"], "query": { "term": { "status": "paid" } }, "sort": [{ "id": "asc" }] }
JSON

# which ES index/alias the raw query hits
cat > e2e/cases/my_case/queries/paid_orders/target.yaml <<'YAML'
index: orders
description: only paid orders
YAML
```

**Step 3 — generate the goldens and run.**

```bash
make -C e2e e2e-update-golden CASE=my_case   # writes golden.ddn.json + golden.es.json
git add e2e/cases/my_case                      # commit the case + its goldens
make -C e2e e2e-case CASE=my_case              # verify it passes
```

That's it — the runner discovers `my_case` automatically. No harness changes.

## Case file reference

Everything except `queries/*/query.graphql`, `es_search.json`, and `target.yaml`
is optional. Seed inputs are applied **in this order** into the fresh ES:

| Path | Effect |
|---|---|
| `indices/<index>.mapping.json` | `PUT /<index>` — the mapping is the schema; the file base name is the index name |
| `init/*.http` / `init/*.sh` | arbitrary ES setup **before** data loads (aliases, ingest pipelines, settings) — see below |
| `data/*.ndjson` | `POST /_bulk`, then `_refresh` the touched indices |
| `case.yaml` → `kibana_sample: logs\|ecommerce\|flights` | load an elastic.co sample dataset via Kibana |

> Ordering note: `init/` runs **before** `data/` on purpose, so ingest
> pipelines/aliases a case declares are already in place when documents are
> indexed.

`init/*.http` is a tiny request DSL — `METHOD /path` followed by an optional JSON
body, requests separated by blank lines, `#` comments allowed:

```
# create an ingest pipeline, then an alias
PUT /_ingest/pipeline/my-pipeline
{ "processors": [ { "set": { "field": "seen", "value": true } } ] }

POST /_aliases
{ "actions": [ { "add": { "index": "orders", "alias": "orders_v1" } } ] }
```

`init/*.sh` scripts run with `ES_URL`, `ES_USER`, `ES_PASS`, `ES_CACERT`
exported (use `curl --cacert "$ES_CACERT" -u "$ES_USER:$ES_PASS" ...`).

Per-query files:

| File | Required | Purpose |
|---|---|---|
| `query.graphql` | ✅ | GraphQL query POSTed to the DDN engine (as role `admin`) |
| `es_search.json` | ✅ | equivalent ES DSL POSTed to `/<target.index>/_search` |
| `target.yaml` | ✅ | `index:` (ES index/alias for the raw search) + `description:` |
| `variables.json` | optional | GraphQL variables |
| `golden.ddn.json` | committed | expected DDN result (regenerate with `UPDATE_GOLDEN=1`) |
| `golden.es.json` | committed | expected ES result (debugging reference; shown alongside DDN on failure) |

A golden may be the sentinel `{"__pending__": true}`; its comparison is
**skipped** (both queries still run and their payloads are recorded in the report)
until you regenerate it.

Per-case files (alongside `indices/`, `queries/`, …):

| File | Required | Purpose |
|---|---|---|
| `golden.schema.json` | committed | snapshot of the connector's full `GET /schema` for the case (regenerate with `UPDATE_GOLDEN=1`); the same `{"__pending__": true}` sentinel skips its comparison |

## Assertions in detail

**Schema golden** (per case): snapshot the connector's entire `GET /schema`
document and compare it against the committed `golden.schema.json`. Unlike L3
(which checks *conformance* to the live ES mapping and is invariant under
object-type renames), this is a **snapshot** of the generated NDC schema surface,
so any change to it — an object-type rename, a new/dropped type, a changed field
type — surfaces as a reviewable golden diff instead of passing silently. Because
the connector emits `collections`/`functions`/`procedures` in nondeterministic
Go-map order, the snapshot is canonicalised (those arrays sorted by name; all
object keys are already sorted by the JSON encoder) before writing/comparing, so
the golden is stable. Regenerate with `UPDATE_GOLDEN=1` after an intended schema
change, then review and commit the diff.

**L3 — schema conformance** (per seeded index):

- `GET /<index>/_mapping` from ES **deep-equals**
  `configuration.json.indices.<index>.mappings`.
- The connector's `GET :8080/schema` has a **collection** for the index, and
  every **leaf** ES field maps to an NDC field whose scalar type carries the
  expected representation per the fixed ES→NDC table in
  [`harness/typemap.go`](harness/typemap.go) (derived from
  [`connector/schema.go`](../connector/schema.go) and
  [`internal/static_types.go`](../internal/static_types.go)):
  `keyword/text/ip/version/date → string`, `long → int64`, `integer → int32`,
  `short → int16`, `byte → int8`, `unsigned_long → biginteger`,
  `float/half_float → float32`, `double/scaled_float → float64`,
  `boolean → boolean`, `geo_point/geo_shape/*_range/… → json`. Multi-fields
  become compound scalar types (`text.keyword`). Every ES object container —
  explicit `object`, `nested`, and implicit-object (properties with no `type`) —
  becomes an **array of a named NDC object type** (the connector models all of
  them uniformly, since an ES object field can hold a single object or an array
  of objects).

**L4 — query parity + goldens** (per query):

- POST the GraphQL to the engine `:3000/graphql` and the DSL to
  `:9200/<target.index>/_search`.
- Both results are compared against their committed golden files
  (`golden.ddn.json` and `golden.es.json`) using JSON deep-equality after
  normalisation (whitespace, key ordering). Array order is preserved — if a
  query specifies `order_by`, the golden must reflect that order.
- On the first run for a new case, generate the goldens with `UPDATE_GOLDEN=1`
  (the harness writes the live results), inspect them, then commit them.

Two structural differences between DDN and ES responses are expected and normal
(they are not bugs):
- **camelCase vs snake_case**: the connector normalises ES field names
  (`in_stock` → `inStock`). The DDN golden uses camelCase, the ES golden uses
  snake_case.
- **Object fields as arrays**: the connector models every ES object container
  (explicit `object`, `nested`, or implicit-object) as an array of a named NDC
  object type. A field whose raw ES value is `{"dest":"US"}` will appear as
  `[{"dest":"US"}]` in the DDN response. The ES golden retains the raw shape.

## Environment variables

| Var | Default | Meaning |
|---|---|---|
| `E2E` | – | must be `1` to run the suite (in addition to the `e2e` build tag) |
| `HASURA_DDN_PAT` | – | DDN Personal Access Token; harness runs `ddn auth login --access-token` with it (unset ⇒ assumes you already `ddn auth login`'d) |
| `UPDATE_GOLDEN` | – | `1` = (re)generate goldens instead of comparing |
| `FAIL_FAST` | off | `1` = `go test -failfast` (on in CI, off locally) |
| `KEEP_STACK` | off | `1` = don't tear a case's stack down (debugging) |
| `E2E_CASE` | – | restrict to one case (set by `make e2e-case CASE=`) |
| `STACK_VERSION` | `8.13.4` | Elasticsearch/Kibana/connector stack version |

## Known quirks

- **Kibana sample data is a data stream.** It lands as `.ds-*` backing indices
  behind the alias `kibana_sample_data_logs`. The connector introspects it under
  the pattern key `".ds-kibana_sample_data_logs-*"`, so the reference case's
  `target.index` uses the **alias** `kibana_sample_data_logs` for the raw
  `_search`.
- **ES TLS is on.** The harness copies the CA cert out of the running `es01`
  container and uses it for all seeding and `_search` calls.
- **Host vs docker connector URL.** Introspection (`ddn connector-link update`)
  runs from the test host and reaches the connector at `localhost:<port>`; the
  engine (in the compose network) reaches it at `http://ndc-elasticsearch:8080`.
  The harness introspects with the host URL, then rewrites the connector host in
  the generated DDN env to the docker URL before `supergraph build local`. This
  is the only mechanical transform of the generated metadata — models themselves
  are never hand-edited.

## First-run checklist for maintainers

Both included cases ship with real, committed goldens and pass out of the box:

- **`custom_products`** — fully-custom case with deterministic data.
- **`kibana_sample_logs`** — Kibana sample logs dataset; 11 query families
  (select, filter term/terms/match/range/and/or/not, sort, pagination,
  aggregation).

The CI workflow is at `.github/workflows/e2e.yaml`. It requires a
`HASURA_DDN_PAT` repository secret. No other secrets are needed.
