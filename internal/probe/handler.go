package probe

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"

	"github.com/steigr/elasticsearch-query-exporter/internal/esquery"
)

const defaultTimeField = "@timestamp"

type Handler struct {
	ES     *esquery.Client
	Store  *AfterTimeStore
	Logger *slog.Logger
}

func NewHandler(es *esquery.Client, store *AfterTimeStore, logger *slog.Logger) *Handler {
	return &Handler{ES: es, Store: store, Logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	receivedAt := time.Now()
	q := r.URL.Query()

	indexPattern := q.Get("index-pattern")
	pattern := q.Get("search-string")
	if indexPattern == "" || pattern == "" {
		http.Error(w, "index-pattern and search-string are required", http.StatusBadRequest)
		return
	}

	field := q.Get("field")
	if field == "" {
		field = "*"
	}
	patternType := esquery.PatternQueryString
	if q.Get("pattern_type") == string(esquery.PatternRegexp) {
		patternType = esquery.PatternRegexp
	}
	timeField := q.Get("time-field")
	if timeField == "" {
		timeField = defaultTimeField
	}
	metricName := q.Get("metric_name")
	if metricName == "" {
		metricName = "elasticsearch_query_result"
	}
	queryIDParam := q.Get("query_id")

	labelFieldMap, err := ParseLabelFieldMap(q["label_field_map"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sourceFields := make([]string, len(labelFieldMap))
	for i, m := range labelFieldMap {
		sourceFields[i] = m.Field
	}

	searchReq := esquery.SearchRequest{
		IndexPattern: indexPattern,
		Pattern:      pattern,
		PatternType:  patternType,
		Field:        field,
		TimeField:    timeField,
		SourceFields: sourceFields,
	}

	id := QueryID(searchReq.String(), labelFieldMap, queryIDParam)

	queryAfter, known := h.Store.Lookup(id)
	if !known {
		h.Store.Set(id, receivedAt)
		http.Error(w, "no data available yet for this query; try again on the next scrape", http.StatusNotFound)
		return
	}

	searchReq.After = queryAfter
	searchReq.Before = receivedAt

	hits, err := h.ES.Search(r.Context(), searchReq)
	if err != nil {
		h.Logger.Error("elasticsearch query failed", "query_id", id, "error", err)
		http.Error(w, fmt.Sprintf("elasticsearch query failed: %v", err), http.StatusBadGateway)
		return
	}

	registry := prometheus.NewRegistry()
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricName,
		Help: "Count of Elasticsearch documents matching the probed query, grouped by label_field_map.",
	}, labelNames(labelFieldMap))
	registry.MustRegister(gauge)

	for _, hit := range hits {
		labels := make(prometheus.Labels, len(labelFieldMap))
		for _, m := range labelFieldMap {
			labels[m.Label] = FieldValue(hit.Source, m.Field)
		}
		gauge.With(labels).Inc()
	}

	metricFamilies, err := registry.Gather()
	if err != nil {
		http.Error(w, fmt.Sprintf("gather metrics: %v", err), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	encoder := expfmt.NewEncoder(&buf, expfmt.NewFormat(expfmt.TypeTextPlain))
	for _, mf := range metricFamilies {
		if err := encoder.Encode(mf); err != nil {
			http.Error(w, fmt.Sprintf("encode metrics: %v", err), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", string(expfmt.NewFormat(expfmt.TypeTextPlain)))
	if _, err := w.Write(buf.Bytes()); err != nil {
		h.Logger.Error("write response failed", "query_id", id, "error", err)
		return
	}

	h.Store.Set(id, receivedAt)
}

func labelNames(m []LabelField) []string {
	names := make([]string, len(m))
	for i, f := range m {
		names[i] = f.Label
	}
	return names
}
