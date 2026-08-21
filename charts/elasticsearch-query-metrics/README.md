# elasticsearch-query-metrics

Renders scrape config — a prometheus-operator `ServiceMonitor` and/or a VictoriaMetrics operator `VMServiceScrape` — for [elasticsearch-query-exporter](../elasticsearch-query-exporter) queries.

This is deliberately a separate chart/release from the exporter itself. Since there's no static `/metrics` endpoint — every query is its own `/probe` scrape config — the set of queries tends to change independently of (and more often than) the exporter's image, resources, or connection settings. Splitting them out means adding, changing, or removing a query is its own release, not a re-release of the exporter Deployment.

## Installing

```bash
helm install elasticsearch-query-metrics \
  oci://ghcr.io/steigr/charts/elasticsearch-query-metrics \
  -f charts/elasticsearch-query-metrics/examples/values-example.yaml
```

Or from a checkout of this repo:

```bash
helm install elasticsearch-query-metrics \
  charts/elasticsearch-query-metrics \
  -f charts/elasticsearch-query-metrics/examples/values-example.yaml
```

## Which CRD gets rendered

Three top-level switches:

| Value | Default | Effect |
|---|---|---|
| `autoDetect` | `true` | Render a `ServiceMonitor` and/or `VMServiceScrape` for whichever CRD is actually installed in the target cluster (checked via `Capabilities.APIVersions`). |
| `prometheus` | `false` | Force-render the `ServiceMonitor`, regardless of `autoDetect`. |
| `victoriaMetrics` | `false` | Force-render the `VMServiceScrape`, regardless of `autoDetect`. |

`autoDetect` needs a live cluster connection to see installed API versions — `helm template` without one sees none, so nothing renders unless you pass `--api-versions monitoring.coreos.com/v1/ServiceMonitor` / `--api-versions operator.victoriametrics.com/v1beta1/VMServiceScrape`, or set `prometheus`/`victoriaMetrics` explicitly.

## Pointing at the exporter's Service

`serviceSelectorLabels` must match the labels on the exporter's `Service` (both CRDs select on it, not by name). The default matches an [elasticsearch-query-exporter](../elasticsearch-query-exporter) chart release named `elasticsearch-query-exporter`:

```yaml
serviceSelectorLabels:
  app.kubernetes.io/name: elasticsearch-query-exporter
  app.kubernetes.io/instance: elasticsearch-query-exporter
```

If you installed that chart under a different release name, or point this at a hand-written Service, update `serviceSelectorLabels` to match.

## Declaring queries

`queries` is a map of query ID to query. The ID doubles as the exporter's `query_id` param (so it must be unique) and feeds the metric name (`elasticsearch_query_<id>`, with `-` replaced by `_`). Every entry becomes one endpoint in every CRD that ends up enabled:

```yaml
queries:
  error-logs:
    indexPattern: "logs-*"
    query: "level:error"
    interval: 60s
    labels:
      service: service.name
      host: host.name
```

- `indexPattern` — Elasticsearch index pattern to search.
- `query` — sent to the exporter as `search-string`.
- `interval` — scrape interval for this query (default `60s` if omitted).
- `labels` — Prometheus label name -> Elasticsearch document field, sent as `label_field_map`.

Each query is independent windowed state in the exporter (see the main [CLAUDE.md](../../CLAUDE.md)) — give two queries distinct IDs even if they share an `indexPattern`/`query`.

Standalone, non-templated examples of the rendered CRDs are in [examples/](examples/): `servicemonitor-single-query.yaml`, `servicemonitor-multi-query.yaml`, `vmservicescrape-multi-query.yaml`.
