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

| Parameter | Required | Description |
|---|---|---|
| `index-pattern` | yes | Elasticsearch index pattern to search. |
| `search-string` | yes | Pattern matched against documents. |
| `pattern_type` | no | `query_string` (default, Lucene syntax/wildcards) or `regexp` (opt-in, more expensive). |
| `field` | no | Field the pattern is matched against (default `*` for `query_string`; required for `regexp`). |
| `time-field` | no | Timestamp field used for windowing (default `@timestamp`). |
| `label_field_map` | repeatable | `label=field` mapping from a Prometheus label to a document field. Last occurrence of a given label wins. |
| `metric_name` | no | Name of the exposed gauge (default `elasticsearch_query_result`). |
| `query_id` | no | Extra identifier to distinguish otherwise-identical queries. |

The exposed metric is a per-label-combination count of documents matched within the scrape's time window. The first scrape of a given query returns `404` (no window yet); subsequent scrapes return `200` with metrics.
