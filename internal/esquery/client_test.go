package esquery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_SendsBasicAuth(t *testing.T) {
	var gotUser, gotPass string
	var gotOK bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, gotOK = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"hits":[]}}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, WithBasicAuth("elastic", "s3cret"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := c.Search(context.Background(), SearchRequest{IndexPattern: "logs-*"}); err != nil {
		t.Fatalf("Search: %v", err)
	}

	if !gotOK || gotUser != "elastic" || gotPass != "s3cret" {
		t.Fatalf("got basic auth (%q, %q, ok=%v), want (elastic, s3cret, true)", gotUser, gotPass, gotOK)
	}
}

func TestClient_NoAuthHeaderWhenUsernameEmpty(t *testing.T) {
	var gotOK bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, gotOK = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":{"hits":[]}}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.Search(context.Background(), SearchRequest{IndexPattern: "logs-*"}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotOK {
		t.Fatal("expected no basic auth header to be sent")
	}
}

func TestWithCACertFile_MissingFile(t *testing.T) {
	if _, err := NewClient("https://example.invalid", WithCACertFile("/nonexistent/ca.crt")); err == nil {
		t.Fatal("expected error for missing CA file")
	}
}
