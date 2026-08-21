package probe

import "testing"

func TestFieldValue(t *testing.T) {
	source := map[string]any{
		"response": map[string]any{
			"status": "500",
		},
		"count": 3.0,
	}

	cases := []struct {
		path string
		want string
	}{
		{"response.status", "500"},
		{"count", "3"},
		{"missing", ""},
		{"response.missing", ""},
		{"response.status.too-deep", ""},
	}
	for _, c := range cases {
		if got := FieldValue(source, c.path); got != c.want {
			t.Errorf("FieldValue(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
