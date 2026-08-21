// Command elasticsearch-query-exporter bridges Elasticsearch searches into
// Prometheus metrics: each scrape of /probe runs one query over the window
// since that same query was last scraped.
package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/steigr/elasticsearch-query-exporter/internal/esquery"
	"github.com/steigr/elasticsearch-query-exporter/internal/probe"
)

func main() {
	listenAddress := flag.String("web.listen-address", ":9206", "Address to listen on for probes.")
	elasticsearchURL := flag.String("elasticsearch.url", "http://localhost:9200", "Elasticsearch base URL.")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	esClient := esquery.NewClient(*elasticsearchURL)
	store := probe.NewAfterTimeStore()
	handler := probe.NewHandler(esClient, store, logger)

	mux := http.NewServeMux()
	mux.Handle("/probe", handler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Elasticsearch Query Exporter</title></head>
<body><h1>Elasticsearch Query Exporter</h1><p><a href="/probe">/probe</a></p></body></html>`))
	})

	logger.Info("starting elasticsearch-query-exporter", "listen-address", *listenAddress, "elasticsearch.url", *elasticsearchURL)
	if err := http.ListenAndServe(*listenAddress, mux); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
