# elasticsearch-query-exporter

A Prometheus exporter that bridges Elasticsearch searches into metrics. Prometheus scrapes a single `/probe` endpoint per query, passing the index pattern, search string, and label/field mapping as query parameters — similar in shape to `blackbox_exporter`.

Consecutive scrapes of the same query are windowed: the exporter tracks, per query, the time range already covered and only asks Elasticsearch for documents added since the last successful scrape. See [CLAUDE.md](CLAUDE.md) for the full design.

## Build

```bash
go build ./...
go test ./...
go run ./cmd/elasticsearch-query-exporter -elasticsearch.url=http://localhost:9200
```

## Container image

Multiarch (`linux/amd64`, `linux/arm64`) image built from the [Dockerfile](Dockerfile):

```bash
hack/build-image.sh ghcr.io/steigr/elasticsearch-query-exporter:latest --push
```

CI builds and publishes the image on every push to `main` and on version tags — see [.github/workflows/container-image.yml](.github/workflows/container-image.yml).

## Deploying

Two Helm charts, published as OCI artifacts under `oci://ghcr.io/steigr/charts/`:

- [charts/elasticsearch-query-exporter](charts/elasticsearch-query-exporter) — the exporter Deployment/Service, with defaults for running next to an [ECK](https://www.elastic.co/guide/en/cloud-on-k8s/current/index.html)-managed Elasticsearch cluster.
- [charts/elasticsearch-query-metrics](charts/elasticsearch-query-metrics) — `ServiceMonitor`/`VMServiceScrape` scrape config for individual queries, installed as its own release so adding or changing a query doesn't require re-releasing the exporter itself.

See each chart's README for details.

## Query parameters

All parameters are kebab-case.

| Parameter | Required | Description |
|---|---|---|
| `index-pattern` | yes | Elasticsearch index pattern to search. |
| `search-string` | yes | Pattern matched against documents (full-text; see `field` for which field). |
| `pattern-type` | no | `query_string` (default, Lucene syntax/wildcards) or `regexp` (opt-in, more expensive). |
| `field` | no | Field `search-string` is matched against (default `*` for `query_string`; required for `regexp`). |
| `time-field` | no | Timestamp field used for windowing (default `@timestamp`). |
| `document-field-filter` | repeatable | `field=value` filter requiring an exact document field to match a value (itself query_string syntax, so `application*` is a valid value). ANDed with `search-string` and with every other filter. Last occurrence of a given field wins. |
| `label-field-map` | repeatable | `label=field` mapping from a Prometheus label to a document field. Last occurrence of a given label wins. |
| `metric-name` | no | Name of the exposed gauge (default `elasticsearch_query_result`). |
| `query-id` | no | Extra identifier to distinguish otherwise-identical queries. |

For example, to search for the full-text term `error` only in documents where `kubernetes.namespace` matches `application*`:

```
/probe?index-pattern=logs-*&search-string=error&document-field-filter=kubernetes.namespace=application*
```

The exposed metric is a per-label-combination count of documents matched within the scrape's time window. The first scrape of a given query returns `404` (no window yet); subsequent scrapes return `200` with metrics.

## Logging

Logs are structured JSON on stdout, field names following the [Elastic Common Schema](https://www.elastic.co/guide/en/ecs/current/index.html) (`@timestamp`, `log.level`, `message`, `error.message`, `ecs.version`, ...) — ready to ship straight into an Elasticsearch/Kibana log pipeline. Level is set with `-log.level` (or `LOG_LEVEL`): `debug`, `info` (default), `warn`, or `error`.

Every `/probe` request logs a summary at `info` (`warn` for a `400`, `error` for a `5xx`) with `http.response.status_code`, `event.duration` (nanoseconds), and `elasticsearch.query.id`. At `-log.level=debug`, the exporter additionally logs the exact query body sent to Elasticsearch for every request (`http.request.body.content`) — copy it straight into Kibana Dev Tools or `curl` to check results by hand.
