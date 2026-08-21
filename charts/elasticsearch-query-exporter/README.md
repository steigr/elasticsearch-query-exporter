# elasticsearch-query-exporter

Helm chart for the [elasticsearch-query-exporter](../../README.md) — a Prometheus exporter that bridges Elasticsearch searches into metrics via a `/probe` endpoint.

## Installing

Published as an OCI artifact on GHCR — no `helm repo add` needed:

```bash
helm install elasticsearch-query-exporter oci://ghcr.io/steigr/charts/elasticsearch-query-exporter \
  -f charts/elasticsearch-query-exporter/examples/values-eck.yaml
```

Or from a checkout of this repo, next to an ECK-managed cluster:

```bash
helm install elasticsearch-query-exporter charts/elasticsearch-query-exporter \
  -f charts/elasticsearch-query-exporter/examples/values-eck.yaml
```

CI publishes the chart to `oci://ghcr.io/steigr/charts/elasticsearch-query-exporter` on every push to `main` (and `v*` tags) that touches `charts/elasticsearch-query-exporter/**` — see [.github/workflows/helm-chart.yml](../../.github/workflows/helm-chart.yml). Bump `version` in `Chart.yaml` before a release so the published tag reflects a real chart change.

The chart's `elasticsearch.*` defaults already assume an [ECK](https://www.elastic.co/guide/en/cloud-on-k8s/current/index.html) cluster named `elasticsearch` in the same namespace:

| Setting | Default | Source |
|---|---|---|
| `elasticsearch.url` | `https://elasticsearch-es-internal-http:9200` | ECK's internal HTTP service |
| `elasticsearch.username` | `elastic` | ECK's default superuser |
| `elasticsearch.existingSecret` / `existingSecretPasswordKey` | `elasticsearch-es-elastic-user` / `elastic` | ECK-managed superuser Secret |
| `elasticsearch.tls.existingSecret` / `caCertKey` | `elasticsearch-es-http-certs-internal` / `ca.crt` | ECK-managed HTTP CA Secret |

Adjust these if your cluster has a different name (ECK derives all of the above from `<cluster-name>-es-*`).

## ServiceMonitor / VMServiceScrape

Two scrape-config CRDs are supported, both driven by the same `serviceMonitor.queries` list:

1. **prometheus-operator `ServiceMonitor`** — set `serviceMonitor.enabled: true`; the chart renders one `ServiceMonitor` with one endpoint per query. See `examples/values-eck.yaml`.
2. **VictoriaMetrics operator `VMServiceScrape`** — set `vmServiceScrape.enabled: true`; renders a `VMServiceScrape` from the *same* `serviceMonitor.queries` list (so the queries only need to be declared once, even with both enabled).

Standalone (non-templated) examples of each are in `examples/`: `servicemonitor-single-query.yaml`, `servicemonitor-multi-query.yaml`, and `vmservicescrape-multi-query.yaml`.

Each query is independent state in the exporter (see the main [CLAUDE.md](../../CLAUDE.md) for the windowing model) — give each a distinct `query_id` if the same `index-pattern`/`search-string` needs multiple label mappings.

## Values

See [values.yaml](values.yaml) for the full set of options (image, resources, ingress, HTTPRoute, autoscaling, extra env/args, etc.) — standard chart conventions from `helm create`, plus the `elasticsearch.*`, `serviceMonitor.*`, and `vmServiceScrape.*` blocks described above.
