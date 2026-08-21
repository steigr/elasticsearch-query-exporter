package esquery

import (
	"encoding/json"
	"testing"
)

// TestSearchRequest_QueryStringUsesDefaultField guards against regressing to
// Elasticsearch's query_string DSL with a "field" option, which doesn't
// exist and produces "[query_string] query does not support [field]" — the
// option is "default_field".
func TestSearchRequest_QueryStringUsesDefaultField(t *testing.T) {
	req := SearchRequest{
		IndexPattern: "logs-*",
		Pattern:      "error",
		PatternType:  PatternQueryString,
		Field:        "*",
	}

	var body struct {
		Query struct {
			Bool struct {
				Must []struct {
					QueryString struct {
						Query        string `json:"query"`
						DefaultField string `json:"default_field"`
						Field        string `json:"field"`
					} `json:"query_string"`
				} `json:"must"`
			} `json:"bool"`
		} `json:"query"`
	}
	if err := json.Unmarshal([]byte(req.String()), &body); err != nil {
		t.Fatalf("unmarshal built query: %v", err)
	}

	if len(body.Query.Bool.Must) != 1 {
		t.Fatalf("expected exactly one must clause, got %d", len(body.Query.Bool.Must))
	}
	qs := body.Query.Bool.Must[0].QueryString
	if qs.DefaultField != "*" {
		t.Errorf("default_field = %q, want %q", qs.DefaultField, "*")
	}
	if qs.Field != "" {
		t.Errorf("query_string must not set the nonexistent \"field\" option, got %q", qs.Field)
	}
	if qs.Query != "error" {
		t.Errorf("query = %q, want %q", qs.Query, "error")
	}
}
