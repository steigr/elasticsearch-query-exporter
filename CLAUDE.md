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

hack/build-image.sh                          # build multiarch (linux/amd64, linux/arm64) container image, --load by default
hack/build-image.sh <image:tag> --push       # build and push a multiarch image
helm lint charts/elasticsearch-query-exporter
helm template <release> charts/elasticsearch-query-exporter -f charts/elasticsearch-query-exporter/examples/values-eck.yaml
```

No external services are required to run the test suite — Elasticsearch interactions are exercised against an `httptest.Server` stub (see `internal/probe/handler_test.go` and `internal/esquery/client_test.go`).

`hack/build-image.sh` needs a `docker buildx` builder that supports both target platforms (`docker buildx create --name multiarch-builder --use`); CI (`.github/workflows/container-image.yml`) builds and pushes to `ghcr.io/steigr/elasticsearch-query-exporter` on pushes to `main` and on `v*` tags.

## Architecture

This is a Prometheus exporter, structured like `blackbox_exporter`: a single `/probe` endpoint that Prometheus scrapes with query parameters describing what to check, rather than a static `/metrics` endpoint. Each scrape is a self-contained request/response — there is no background scraping loop.

### The stateful windowing model (the core design constraint)

The exporter's defining behavior is that consecutive scrapes of the *same logical query* are stitched into non-overlapping time windows, so Elasticsearch is queried incrementally instead of Prometheus re-scanning the same documents every scrape:

1. On each `/probe` request, the received time is recorded first (`handler.go`, `receivedAt`).
2. An **internal query ID** is derived from the tuple `(elasticsearch query JSON, label-field-map, query-id param)` via `probe.QueryID` — see `queryid.go`. Two scrapes are "the same query" only if all three match exactly. This ID is intentionally computed from the *serialized ES query* (`esquery.SearchRequest.String()`), not the raw `search-string` param, so that pattern type, target field, time field, and `document-field-filter` are all part of identity (they're baked into the query JSON) without needing separate handling — only `label-field-map`, which doesn't touch the ES query at all, has to be threaded into `QueryID` explicitly.
3. `probe.AfterTimeStore` (`store.go`) is a global, in-memory, mutex-guarded `map[queryID]time.Time` — the *only* state the process keeps. It survives only in-process; there is no persistence across restarts.
4. **First time an ID is seen:** store `receivedAt` as its "query after time" and respond `404`. This is intentional — there is no valid window yet, so Prometheus should treat the scrape as having no data rather than an error.
5. **Subsequent scrapes of the same ID:** query Elasticsearch over `(query_after_time, receivedAt]` (exclusive lower bound, inclusive upper bound), then advance the stored "query after time" to `receivedAt` — but only *after* a successful response is written. A failed Elasticsearch query or encoding error must not advance the window, or documents in that window would be silently skipped on the next scrape.

When modifying the probe flow, preserve this ordering: capture time → resolve ID → look up window → query → respond → *then* advance stored state.

### Package layout

- `cmd/elasticsearch-query-exporter/` — flag parsing and HTTP server wiring only.
- `internal/probe/` — everything scrape-shaped and stateful: the `/probe` handler, the query-ID derivation, `label-field-map`/`document-field-filter` parsing (`labelmap.go`/`fieldfilter.go`, both built on the shared `parseKeyValuePairs` in `pairs.go`), the after-time store, and dotted-path field resolution (`fields.go`) from ES `_source` documents.
- `internal/esquery/` — everything Elasticsearch-shaped and stateless: builds the query DSL and runs it over the REST API via plain `net/http` (no ES client library). `SearchRequest.String()` is the canonical serialized form of a query and is what feeds the query ID — keep it deterministic.

### Query pattern types

`search-string` is matched one of two ways (`esquery.PatternType`):

- `query_string` (default) — Lucene `query_string` query. Preferred: supports glob-style wildcards and boolean syntax, and can be served from the inverted index.
- `regexp` (opt-in via `pattern-type=regexp`) — Elasticsearch `regexp` query against a single field. Notably more expensive at scale since it can't use the inverted index the same way; only reach for it when `query_string` syntax genuinely can't express the match.

### `document-field-filter`: additional per-field filters

Repeated `field=value` query params (parsed by `probe.ParseFieldFilters`, `fieldfilter.go`), each becoming a `query_string` clause scoped to that field (`default_field`) in the ES query's `bool.filter` array — i.e. ANDed with the main `search-string` match and with every other filter, in filter context (no effect on scoring). Because `Value` is itself `query_string` syntax, filters support the same wildcards as the main pattern (`kubernetes.namespace=application*`), not just exact terms. This is what `esquery.SearchRequest.FieldFilters` carries — see `buildQuery` in `client.go`. Unlike `label-field-map`, filters live entirely inside the ES query and therefore don't need separate handling in `probe.QueryID` — `esquery.SearchRequest.String()` already picks them up.

### Metrics generation

Each `/probe` response builds a fresh `prometheus.Registry` (not the global default registry) scoped to that single request, registers one `GaugeVec` named by `metric-name` (default `elasticsearch_query_result`) with label names taken from `label-field-map`, and increments it once per matching document — i.e. the exposed value is a per-label-combination count of documents matched within the window, not the raw document count. Label values come from `probe.FieldValue`, which resolves a dotted path against the document's `_source` and returns `""` for any missing segment (per spec: "the label value is the field value or an empty string").

### `label-field-map` semantics

Passed as repeated query params in `label=field` form. `ParseLabelFieldMap` (`labelmap.go`) deduplicates by label name — **last occurrence wins** — then sorts by label name for deterministic ordering, since that ordering feeds directly into the query ID hash. `document-field-filter` (`field=value`) shares the same dedup/sort/error-message machinery via `parseKeyValuePairs` (`pairs.go`) — extend that shared helper, not either caller individually, if the parsing rules ever need to change.

All query parameter names are kebab-case (`index-pattern`, `pattern-type`, `label-field-map`, `document-field-filter`, `metric-name`, `query-id`, ...) — keep new ones consistent with that when adding parameters.

### Logging

`internal/ecslog` (`ecslog.go`) wraps `slog.NewJSONHandler` with a `ReplaceAttr` that renames the built-in `time`/`level`/`msg` keys to ECS's `@timestamp`/`log.level`/`message`, lowercases the level string, and stamps every record with `ecs.version` via `.With(...)` on the base logger. `ecslog.ParseLevel` maps `-log.level`/`LOG_LEVEL` (`debug`/`info`/`warn`/`error`) to an `slog.Level`; `ecslog.Err(err)` renders an error as ECS's `error.message` group — use it instead of a bare `"error", err` attr so error logging stays consistent (and doesn't collide with `ReplaceAttr`, which only rewrites the three built-in keys — never name a custom attribute `time`, `level`, or `msg`, or it gets silently renamed too since `ReplaceAttr` can't tell a same-named custom attr from the built-in one).

`probe.Handler.ServeHTTP` wraps the `http.ResponseWriter` in a `statusRecorder` (`handler.go`) purely to capture the status code for a deferred per-request summary log (`"handled probe request"`: method, path, query, status, `event.duration` in nanoseconds, `elasticsearch.query.id`) — level is chosen by `levelForStatus` (5xx → Error, 400 → Warn, everything else including the routine first-scrape 404 → Info). `esquery.Client.Search` logs the exact outgoing query at Debug (`"sending elasticsearch query"`, `http.request.body.content` is the literal JSON body) via an optional `Logger` field (`WithLogger` option, falls back to a discarding logger if unset so tests don't need to wire one up) — this is what `-log.level=debug` is for: capture that field's value and replay it directly against the cluster to debug a query.

### Elasticsearch client auth/TLS

`esquery.NewClient` takes functional options (`WithBasicAuth`, `WithCACertFile`, `WithInsecureSkipVerify`) rather than a fixed config struct, so `main.go` only wires in what's configured. `main.go`'s flags (`-elasticsearch.username`, `-elasticsearch.password`, `-elasticsearch.ca-file`, ...) each fall back to an env var default (`ELASTICSEARCH_USERNAME`, etc.) via `envDefault`, so credentials can come from a mounted Secret's env var without a flag ever appearing in `ps`/process args — keep that split (URL/username as flags, password as env-var-only in the Helm chart) when adding new secret-shaped settings.

### Container image

`Dockerfile` is a multi-stage cross-compiling build (`GOOS`/`GOARCH` set from buildx's `TARGETOS`/`TARGETARCH`, not QEMU-emulated compilation) onto `gcr.io/distroless/static-debian13:nonroot`. The builder's Go version in the Dockerfile must track `go.mod`'s `go` directive — a mismatch fails the build with "go.mod requires go >= ...".

### Helm charts

Two charts, both scaffolded with `helm create` (Helm 3+'s replacement for the removed `helm init`) and then adjusted, published independently to `oci://ghcr.io/steigr/charts/<name>` by the same CI job (`.github/workflows/helm-chart.yml`, which loops over `charts/*/`).

- **`charts/elasticsearch-query-exporter`** — the exporter Deployment/Service only. `elasticsearch.*` values default to an [ECK](https://www.elastic.co/guide/en/cloud-on-k8s/current/index.html)-managed cluster named `elasticsearch`: HTTPS URL to `elasticsearch-es-internal-http:9200`, username `elastic`, password from Secret `elasticsearch-es-elastic-user` key `elastic` (mounted as the `ELASTICSEARCH_PASSWORD` env var, never a flag), and the HTTP CA from Secret `elasticsearch-es-http-certs-internal` key `ca.crt` (mounted as a file, referenced via `-elasticsearch.ca-file`). `deployment.yaml` conditionally adds the CA volume/mount/arg only when `elasticsearch.tls.existingSecret` is set. It has **no** query configuration and renders no scrape-config CRD.
- **`charts/elasticsearch-query-metrics`** — renders scrape config only: a `ServiceMonitor` (`templates/servicemonitor.yaml`) and/or a VictoriaMetrics `VMServiceScrape` (`templates/vmservicescrape.yaml`), both built from the *same* `queries` map — `queries` is `queryId: {indexPattern, query, interval, labels, filters}`, one endpoint per entry. `query` is the exporter's `search-string`; `labels` is the `label-field-map` (Prometheus label -> ES field); `filters` (optional) is the `document-field-filter` (ES field -> required value); the metric name is derived as `elasticsearch_query_<queryId>` (`-` → `_`) rather than being its own field. Deliberately a separate chart/release from the exporter: since there's no static `/metrics` path, every query is its own scrape config, and that set changes independently of (and more often than) the exporter's image/resources/connection settings — splitting them out means a query change isn't a Deployment re-release. It doesn't own a Service; `serviceSelectorLabels` in its values must match the exporter chart's Service labels (defaults assume a release named `elasticsearch-query-exporter`).

  Which CRD(s) render is controlled by three top-level booleans, not per-template enable flags: `prometheus` and `victoriaMetrics` (both default `false`) force-render their respective CRD regardless of anything else; `autoDetect` (default `true`) additionally renders whichever CRD's API group is present in `.Capabilities.APIVersions` — real `helm install`/`upgrade` sees this from the live cluster, but plain `helm template` sees none unless given `--api-versions`, so an unmodified `helm template` on this chart renders nothing by default. Both templates independently OR their own force-flag with the autoDetect check (`$enabled := or .Values.prometheus (and .Values.autoDetect (.Capabilities.APIVersions.Has "monitoring.coreos.com/v1/ServiceMonitor"))`, mirrored for VM) — when adding a third CRD type, follow that same pattern rather than a shared enable list, since each CRD's presence is detected independently. `examples/` in each chart has values-driven and hand-written standalone manifests.

When changing either chart's `templates/` or `values.yaml`, bump that chart's `version` in its `Chart.yaml` — CI publishes on push regardless, but an unbumped version silently overwrites the same OCI tag.
