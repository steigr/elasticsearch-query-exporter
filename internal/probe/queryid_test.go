package probe

import "testing"

func TestQueryID_Deterministic(t *testing.T) {
	m := []LabelField{{Label: "status", Field: "response.status"}}
	id1 := QueryID("es-query", m, "custom")
	id2 := QueryID("es-query", m, "custom")
	if id1 != id2 {
		t.Fatalf("expected deterministic ID, got %q and %q", id1, id2)
	}
}

func TestQueryID_DiffersOnAnyComponent(t *testing.T) {
	base := QueryID("es-query", []LabelField{{Label: "a", Field: "f"}}, "id")

	cases := map[string]string{
		"different query string": QueryID("other-query", []LabelField{{Label: "a", Field: "f"}}, "id"),
		"different label map":    QueryID("es-query", []LabelField{{Label: "a", Field: "g"}}, "id"),
		"different query_id":     QueryID("es-query", []LabelField{{Label: "a", Field: "f"}}, "other"),
	}
	for name, other := range cases {
		if other == base {
			t.Errorf("%s: expected different query ID, got same as base", name)
		}
	}
}
