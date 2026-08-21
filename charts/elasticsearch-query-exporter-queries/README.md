# elasticsearch-query-exporter-queries

Renders scrape config — a prometheus-operator `ServiceMonitor` and/or a VictoriaMetrics operator `VMServiceScrape` — for [elasticsearch-query-exporter](../elasticsearch-query-exporter) queries.

This is deliberately a separate chart/release from the exporter itself. Since there's no static `/metrics` endpoint — every query is its own `/probe` scrape config — the set of queries tends to change independently of (and more often than) the exporter's image, resources, or connection settings. Splitting them out means adding, changing, or removing a query is its own release, not a re-release of the exporter Deployment.

Both CRDs are **disabled by default**; enable whichever your monitoring stack uses.

## Installing

```bash
helm install elasticsearch-query-exporter-queries \
  oci://ghcr.io/steigr/charts/elasticsearch-query-exporter-queries \
  -f charts/elasticsearch-query-exporter-queries/examples/values-example.yaml
```

Or from a checkout of this repo:

```bash
helm install elasticsearch-query-exporter-queries \
  charts/elasticsearch-query-exporter-queries \
  -f charts/elasticsearch-query-exporter-queries/examples/values-example.yaml
```

## Pointing at the exporter's Service

`serviceSelectorLabels` must match the labels on the exporter's `Service` (both CRDs select on it, not by name). The default matches an [elasticsearch-query-exporter](../elasticsearch-query-exporter) chart release named `elasticsearch-query-exporter`:

```yaml
serviceSelectorLabels:
  app.kubernetes.io/name: elasticsearch-query-exporter
  app.kubernetes.io/instance: elasticsearch-query-exporter
```

If you installed that chart under a different release name, or point this at a hand-written Service, update `serviceSelectorLabels` to match.

## Declaring queries

Every entry in `queries` becomes one endpoint in every enabled CRD:

```yaml
serviceMonitor:
  enabled: true

queries:
  - name: error-logs
    indexPattern: "logs-*"
    searchString: "level:error"
    timeField: "@timestamp"
    metricName: "elasticsearch_error_log_count"
    queryId: "error-logs"
    labelFieldMap:
      service: service.name
      host: host.name
```

See [values.yaml](values.yaml) for every field (per-query `interval`/`scrapeTimeout` overrides, `metricRelabelings` for `ServiceMonitor`, `metricRelabelConfigs` for `VMServiceScrape`). Give each query a distinct `queryId` if the same `indexPattern`/`searchString` needs multiple label mappings — each is independent windowed state in the exporter (see the main [CLAUDE.md](../../CLAUDE.md)).

Standalone, non-templated examples of the rendered CRDs are in [examples/](examples/): `servicemonitor-single-query.yaml`, `servicemonitor-multi-query.yaml`, `vmservicescrape-multi-query.yaml`.
