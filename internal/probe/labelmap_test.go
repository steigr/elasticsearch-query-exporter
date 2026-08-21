package probe

import (
	"reflect"
	"testing"
)

func TestParseLabelFieldMap_LastOccurrenceWins(t *testing.T) {
	got, err := ParseLabelFieldMap([]string{"status=response.code", "status=response.status_text"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []LabelField{{Label: "status", Field: "response.status_text"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseLabelFieldMap_SortedByLabel(t *testing.T) {
	got, err := ParseLabelFieldMap([]string{"b=field2", "a=field1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []LabelField{{Label: "a", Field: "field1"}, {Label: "b", Field: "field2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseLabelFieldMap_InvalidFormat(t *testing.T) {
	if _, err := ParseLabelFieldMap([]string{"no-equals-sign"}); err == nil {
		t.Fatal("expected error for malformed entry")
	}
}
