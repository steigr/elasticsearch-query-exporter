# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...                              # build everything
go build -o bin/elasticsearch-query-exporter ./cmd/elasticsearch-query-exporter
go run ./cmd/elasticsearch-query-exporter    # run locally (flags: -web.listen-address, -elasticsearch.url)
go test ./...                                # run all tests
go test ./internal/probe/ -run TestQueryID   # run a single test
go vet ./...
gofmt -l .                                   # list unformatted files (should be empty)
```

No external services are required to run the test suite — Elasticsearch interactions are exercised against an `httptest.Server` stub (see `internal/probe/handler_test.go`).

## Architecture

This is a Prometheus exporter, structured like `blackbox_exporter`: a single `/probe` endpoint that Prometheus scrapes with query parameters describing what to check, rather than a static `/metrics` endpoint. Each scrape is a self-contained request/response — there is no background scraping loop.

### The stateful windowing model (the core design constraint)

The exporter's defining behavior is that consecutive scrapes of the *same logical query* are stitched into non-overlapping time windows, so Elasticsearch is queried incrementally instead of Prometheus re-scanning the same documents every scrape:

1. On each `/probe` request, the received time is recorded first (`handler.go`, `receivedAt`).
2. An **internal query ID** is derived from the tuple `(elasticsearch query JSON, label_field_map, query_id param)` via `probe.QueryID` — see `queryid.go`. Two scrapes are "the same query" only if all three match exactly. This ID is intentionally computed from the *serialized ES query* (`esquery.SearchRequest.String()`), not the raw `search-string` param, so that pattern type, target field, and time field are all part of identity.
3. `probe.AfterTimeStore` (`store.go`) is a global, in-memory, mutex-guarded `map[queryID]time.Time` — the *only* state the process keeps. It survives only in-process; there is no persistence across restarts.
4. **First time an ID is seen:** store `receivedAt` as its "query after time" and respond `404`. This is intentional — there is no valid window yet, so Prometheus should treat the scrape as having no data rather than an error.
5. **Subsequent scrapes of the same ID:** query Elasticsearch over `(query_after_time, receivedAt]` (exclusive lower bound, inclusive upper bound), then advance the stored "query after time" to `receivedAt` — but only *after* a successful response is written. A failed Elasticsearch query or encoding error must not advance the window, or documents in that window would be silently skipped on the next scrape.

When modifying the probe flow, preserve this ordering: capture time → resolve ID → look up window → query → respond → *then* advance stored state.

### Package layout

- `cmd/elasticsearch-query-exporter/` — flag parsing and HTTP server wiring only.
- `internal/probe/` — everything scrape-shaped and stateful: the `/probe` handler, the query-ID derivation, the `label_field_map` parsing, the after-time store, and dotted-path field resolution (`fields.go`) from ES `_source` documents.
- `internal/esquery/` — everything Elasticsearch-shaped and stateless: builds the query DSL and runs it over the REST API via plain `net/http` (no ES client library). `SearchRequest.String()` is the canonical serialized form of a query and is what feeds the query ID — keep it deterministic.

### Query pattern types

`search-string` is matched one of two ways (`esquery.PatternType`):

- `query_string` (default) — Lucene `query_string` query. Preferred: supports glob-style wildcards and boolean syntax, and can be served from the inverted index.
- `regexp` (opt-in via `pattern_type=regexp`) — Elasticsearch `regexp` query against a single field. Notably more expensive at scale since it can't use the inverted index the same way; only reach for it when `query_string` syntax genuinely can't express the match.

### Metrics generation

Each `/probe` response builds a fresh `prometheus.Registry` (not the global default registry) scoped to that single request, registers one `GaugeVec` named by `metric_name` (default `elasticsearch_query_result`) with label names taken from `label_field_map`, and increments it once per matching document — i.e. the exposed value is a per-label-combination count of documents matched within the window, not the raw document count. Label values come from `probe.FieldValue`, which resolves a dotted path against the document's `_source` and returns `""` for any missing segment (per spec: "the label value is the field value or an empty string").

### `label_field_map` semantics

Passed as repeated query params in `label=field` form. `ParseLabelFieldMap` (`labelmap.go`) deduplicates by label name — **last occurrence wins** — then sorts by label name for deterministic ordering, since that ordering feeds directly into the query ID hash.
