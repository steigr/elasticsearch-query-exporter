package probe

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"

	"github.com/steigr/elasticsearch-query-exporter/internal/ecslog"
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

// statusRecorder captures the status code written to an http.ResponseWriter
// for logging, since neither Write nor the ResponseWriter interface expose
// it after the fact.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// levelForStatus picks a log level for the per-request summary log: 5xx are
// server-side failures (Error), 400 is a caller mistake (Warn), everything
// else — including the very common first-scrape 404 — is a normal outcome
// (Info).
func levelForStatus(status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status == http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	receivedAt := time.Now()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	var queryID string

	defer func() {
		h.Logger.LogAttrs(r.Context(), levelForStatus(rec.status), "handled probe request",
			slog.String("http.request.method", r.Method),
			slog.String("url.path", r.URL.Path),
			slog.String("url.query", r.URL.RawQuery),
			slog.Int("http.response.status_code", rec.status),
			slog.Int64("event.duration", time.Since(receivedAt).Nanoseconds()),
			slog.String("elasticsearch.query.id", queryID),
		)
	}()

	q := r.URL.Query()

	indexPattern := q.Get("index-pattern")
	pattern := q.Get("search-string")
	if indexPattern == "" || pattern == "" {
		http.Error(rec, "index-pattern and search-string are required", http.StatusBadRequest)
		return
	}

	field := q.Get("field")
	if field == "" {
		field = "*"
	}
	patternType := esquery.PatternQueryString
	if q.Get("pattern-type") == string(esquery.PatternRegexp) {
		patternType = esquery.PatternRegexp
	}
	timeField := q.Get("time-field")
	if timeField == "" {
		timeField = defaultTimeField
	}
	metricName := q.Get("metric-name")
	if metricName == "" {
		metricName = "elasticsearch_query_result"
	}
	queryIDParam := q.Get("query-id")

	labelFieldMap, err := ParseLabelFieldMap(q["label-field-map"])
	if err != nil {
		http.Error(rec, err.Error(), http.StatusBadRequest)
		return
	}

	fieldFilters, err := ParseFieldFilters(q["document-field-filter"])
	if err != nil {
		http.Error(rec, err.Error(), http.StatusBadRequest)
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
		FieldFilters: fieldFilters,
		SourceFields: sourceFields,
	}

	queryID = QueryID(searchReq.String(), labelFieldMap, queryIDParam)

	queryAfter, known := h.Store.Lookup(queryID)
	if !known {
		h.Store.Set(queryID, receivedAt)
		http.Error(rec, "no data available yet for this query; try again on the next scrape", http.StatusNotFound)
		return
	}

	searchReq.After = queryAfter
	searchReq.Before = receivedAt

	hits, err := h.ES.Search(r.Context(), searchReq)
	if err != nil {
		h.Logger.Error("elasticsearch query failed", "elasticsearch.query.id", queryID, ecslog.Err(err))
		http.Error(rec, fmt.Sprintf("elasticsearch query failed: %v", err), http.StatusBadGateway)
		return
	}

	registry := prometheus.NewRegistry()
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricName,
		Help: "Count of Elasticsearch documents matching the probed query, grouped by label-field-map.",
	}, labelNames(labelFieldMap))
	registry.MustRegister(gauge)

	for _, hit := range hits {
		labels := make(prometheus.Labels, len(labelFieldMap))
		for _, m := range labelFieldMap {
			labels[m.Label] = FieldValue(hit.Source, m.Field)
		}
		gauge.With(labels).Inc()
	}

	h.Logger.Debug("elasticsearch query matched documents", "elasticsearch.query.id", queryID, "elasticsearch.hits", len(hits))

	metricFamilies, err := registry.Gather()
	if err != nil {
		http.Error(rec, fmt.Sprintf("gather metrics: %v", err), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	encoder := expfmt.NewEncoder(&buf, expfmt.NewFormat(expfmt.TypeTextPlain))
	for _, mf := range metricFamilies {
		if err := encoder.Encode(mf); err != nil {
			http.Error(rec, fmt.Sprintf("encode metrics: %v", err), http.StatusInternalServerError)
			return
		}
	}

	rec.Header().Set("Content-Type", string(expfmt.NewFormat(expfmt.TypeTextPlain)))
	if _, err := rec.Write(buf.Bytes()); err != nil {
		h.Logger.Error("write response failed", "elasticsearch.query.id", queryID, ecslog.Err(err))
		return
	}

	h.Store.Set(queryID, receivedAt)
}

func labelNames(m []LabelField) []string {
	names := make([]string, len(m))
	for i, f := range m {
		names[i] = f.Label
	}
	return names
}
