package probe

import (
	"reflect"
	"testing"

	"github.com/steigr/elasticsearch-query-exporter/internal/esquery"
)

func TestParseFieldFilters_LastOccurrenceWins(t *testing.T) {
	got, err := ParseFieldFilters([]string{"kubernetes.namespace=staging", "kubernetes.namespace=application*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []esquery.FieldFilter{{Field: "kubernetes.namespace", Value: "application*"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseFieldFilters_Multiple(t *testing.T) {
	got, err := ParseFieldFilters([]string{"b=2", "a=1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []esquery.FieldFilter{{Field: "a", Value: "1"}, {Field: "b", Value: "2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseFieldFilters_InvalidFormat(t *testing.T) {
	if _, err := ParseFieldFilters([]string{"no-equals-sign"}); err == nil {
		t.Fatal("expected error for malformed entry")
	}
}
