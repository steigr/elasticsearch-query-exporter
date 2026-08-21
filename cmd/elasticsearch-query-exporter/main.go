// Command elasticsearch-query-exporter bridges Elasticsearch searches into
// Prometheus metrics: each scrape of /probe runs one query over the window
// since that same query was last scraped.
package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/steigr/elasticsearch-query-exporter/internal/ecslog"
	"github.com/steigr/elasticsearch-query-exporter/internal/esquery"
	"github.com/steigr/elasticsearch-query-exporter/internal/probe"
)

// envDefault returns the value of the environment variable key, or
// fallback if it is unset, for use as a flag's default value.
func envDefault(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func main() {
	listenAddress := flag.String("web.listen-address", ":9206", "Address to listen on for probes.")
	elasticsearchURL := flag.String("elasticsearch.url", envDefault("ELASTICSEARCH_URL", "http://localhost:9200"), "Elasticsearch base URL.")
	elasticsearchUsername := flag.String("elasticsearch.username", envDefault("ELASTICSEARCH_USERNAME", ""), "Elasticsearch basic auth username.")
	elasticsearchPassword := flag.String("elasticsearch.password", envDefault("ELASTICSEARCH_PASSWORD", ""), "Elasticsearch basic auth password.")
	elasticsearchCAFile := flag.String("elasticsearch.ca-file", envDefault("ELASTICSEARCH_CA_FILE", ""), "Path to a PEM-encoded CA certificate to trust in addition to the system pool.")
	elasticsearchInsecureSkipVerify := flag.Bool("elasticsearch.tls-insecure-skip-verify", false, "Disable Elasticsearch TLS certificate verification (testing only).")
	logLevel := flag.String("log.level", envDefault("LOG_LEVEL", "info"), "Log level: debug, info, warn, or error. debug also logs every query sent to Elasticsearch.")
	flag.Parse()

	level, err := ecslog.ParseLevel(*logLevel)
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	logger := ecslog.New(level)

	esOpts := []esquery.Option{
		esquery.WithInsecureSkipVerify(*elasticsearchInsecureSkipVerify),
		esquery.WithLogger(logger),
	}
	if *elasticsearchUsername != "" {
		esOpts = append(esOpts, esquery.WithBasicAuth(*elasticsearchUsername, *elasticsearchPassword))
	}
	if *elasticsearchCAFile != "" {
		esOpts = append(esOpts, esquery.WithCACertFile(*elasticsearchCAFile))
	}

	esClient, err := esquery.NewClient(*elasticsearchURL, esOpts...)
	if err != nil {
		logger.Error("failed to configure elasticsearch client", ecslog.Err(err))
		os.Exit(1)
	}

	store := probe.NewAfterTimeStore()
	handler := probe.NewHandler(esClient, store, logger)

	mux := http.NewServeMux()
	mux.Handle("/probe", handler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Elasticsearch Query Exporter</title></head>
<body><h1>Elasticsearch Query Exporter</h1><p><a href="/probe">/probe</a></p></body></html>`))
	})

	logger.Info("starting elasticsearch-query-exporter",
		"listen-address", *listenAddress,
		"elasticsearch.url", *elasticsearchURL,
		"configured_log_level", level.String(),
	)
	if err := http.ListenAndServe(*listenAddress, mux); err != nil {
		logger.Error("server failed", ecslog.Err(err))
		os.Exit(1)
	}
}
