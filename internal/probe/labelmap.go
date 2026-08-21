package probe

// LabelField maps a Prometheus label name to an Elasticsearch document field.
type LabelField struct {
	Label string
	Field string
}

// ParseLabelFieldMap parses repeated "label=field" label-field-map query
// parameter values into a deduplicated, deterministically ordered mapping.
// When a label name occurs more than once, the last occurrence wins.
func ParseLabelFieldMap(values []string) ([]LabelField, error) {
	pairs, err := parseKeyValuePairs(values, "label-field-map")
	if err != nil {
		return nil, err
	}

	mappings := make([]LabelField, len(pairs))
	for i, p := range pairs {
		mappings[i] = LabelField{Label: p[0], Field: p[1]}
	}
	return mappings, nil
}
