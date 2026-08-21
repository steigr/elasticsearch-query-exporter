package probe

import (
	"fmt"
	"sort"
	"strings"
)

// LabelField maps a Prometheus label name to an Elasticsearch document field.
type LabelField struct {
	Label string
	Field string
}

// ParseLabelFieldMap parses repeated "label=field" query parameter values
// into a deduplicated, deterministically ordered mapping. When a label name
// occurs more than once, the last occurrence wins.
func ParseLabelFieldMap(values []string) ([]LabelField, error) {
	byLabel := make(map[string]string, len(values))
	for _, v := range values {
		label, field, ok := strings.Cut(v, "=")
		if !ok || label == "" {
			return nil, fmt.Errorf("invalid label_field_map %q: expected format label=field", v)
		}
		byLabel[label] = field // last occurrence wins
	}

	mappings := make([]LabelField, 0, len(byLabel))
	for label, field := range byLabel {
		mappings = append(mappings, LabelField{Label: label, Field: field})
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Label < mappings[j].Label })
	return mappings, nil
}
