package probe

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/steigr/elasticsearch-query-exporter/internal/esquery"
)

func newTestHandler(t *testing.T, esResponse string) *Handler {
	t.Helper()
	es := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(esResponse))
	}))
	t.Cleanup(es.Close)

	esClient, err := esquery.NewClient(es.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(esClient, NewAfterTimeStore(), logger)
}

func TestHandler_FirstScrapeReturns404(t *testing.T) {
	h := newTestHandler(t, `{"hits":{"hits":[]}}`)

	req := httptest.NewRequest(http.MethodGet, "/probe?index-pattern=logs-*&search-string=error&label_field_map=status=response.status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_SecondScrapeReturnsMetrics(t *testing.T) {
	h := newTestHandler(t, `{"hits":{"hits":[{"_source":{"response":{"status":"500"}}}]}}`)

	url := "/probe?index-pattern=logs-*&search-string=error&label_field_map=status=response.status"

	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, url, nil))
	if first.Code != http.StatusNotFound {
		t.Fatalf("first scrape: got status %d, want 404", first.Code)
	}

	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodGet, url, nil))
	if second.Code != http.StatusOK {
		t.Fatalf("second scrape: got status %d, want 200; body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), `elasticsearch_query_result{status="500"} 1`) {
		t.Fatalf("unexpected body: %s", second.Body.String())
	}
}

func TestHandler_MissingRequiredParams(t *testing.T) {
	h := newTestHandler(t, `{"hits":{"hits":[]}}`)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", rec.Code)
	}
}
