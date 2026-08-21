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

This chart only deploys the exporter itself — it has no query configuration and renders no `ServiceMonitor` or `VMServiceScrape`. Scrape config for individual queries lives in the separate [elasticsearch-query-metrics](../elasticsearch-query-metrics) chart, installed as its own release pointed at this chart's Service. That split means adding, changing, or removing a query is its own release, not a re-release of this Deployment.

## Logging

`logLevel` (default `info`) sets `-log.level`: `debug`, `info`, `warn`, or `error`. Logs are structured JSON on stdout following the Elastic Common Schema. At `debug`, every request additionally logs the exact query body sent to Elasticsearch — useful for pulling a query out of the logs to replay by hand.

## Values

See [values.yaml](values.yaml) for the full set of options (image, resources, ingress, HTTPRoute, autoscaling, extra env/args, etc.) — standard chart conventions from `helm create`, plus the `elasticsearch.*` block described above and `logLevel`.
