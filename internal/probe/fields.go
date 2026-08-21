package probe

import (
	"fmt"
	"strings"
)

// FieldValue resolves a dotted field path (e.g. "response.status") against a
// parsed Elasticsearch _source document, returning "" if any segment is
// absent — matching the spec's "field value or an empty string" rule.
func FieldValue(source map[string]any, path string) string {
	var cur any = source
	for _, segment := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		v, ok := m[segment]
		if !ok {
			return ""
		}
		cur = v
	}

	switch v := cur.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
