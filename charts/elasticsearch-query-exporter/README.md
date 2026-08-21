# elasticsearch-query-exporter

Helm chart for the [elasticsearch-query-exporter](../../README.md) — a Prometheus exporter that bridges Elasticsearch searches into metrics via a `/probe` endpoint.

## Installing next to an ECK-managed cluster

```bash
helm install elasticsearch-query-exporter charts/elasticsearch-query-exporter \
  -f charts/elasticsearch-query-exporter/examples/values-eck.yaml
```

The chart's `elasticsearch.*` defaults already assume an [ECK](https://www.elastic.co/guide/en/cloud-on-k8s/current/index.html) cluster named `elasticsearch` in the same namespace:

| Setting | Default | Source |
|---|---|---|
| `elasticsearch.url` | `https://elasticsearch-es-internal-http:9200` | ECK's internal HTTP service |
| `elasticsearch.username` | `elastic` | ECK's default superuser |
| `elasticsearch.existingSecret` / `existingSecretPasswordKey` | `elasticsearch-es-elastic-user` / `elastic` | ECK-managed superuser Secret |
| `elasticsearch.tls.existingSecret` / `caCertKey` | `elasticsearch-es-http-certs-internal` / `ca.crt` | ECK-managed HTTP CA Secret |

Adjust these if your cluster has a different name (ECK derives all of the above from `<cluster-name>-es-*`).

## ServiceMonitor

Two ways to scrape queries with the prometheus-operator:

1. **Templated** — set `serviceMonitor.enabled: true` and list queries under `serviceMonitor.queries`; the chart renders one `ServiceMonitor` with one endpoint per query. See `examples/values-eck.yaml`.
2. **Standalone** — write your own `ServiceMonitor` against the exporter's Service. See `examples/servicemonitor-single-query.yaml` and `examples/servicemonitor-multi-query.yaml`.

Each query is independent state in the exporter (see the main [CLAUDE.md](../../CLAUDE.md) for the windowing model) — give each a distinct `query_id` if the same `index-pattern`/`search-string` needs multiple label mappings.

## Values

See [values.yaml](values.yaml) for the full set of options (image, resources, ingress, HTTPRoute, autoscaling, extra env/args, etc.) — standard chart conventions from `helm create`, plus the `elasticsearch.*` and `serviceMonitor.*` blocks described above.
