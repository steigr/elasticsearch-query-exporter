package probe

import "github.com/steigr/elasticsearch-query-exporter/internal/esquery"

// ParseFieldFilters parses repeated "field=value" document-field-filter
// query parameter values into a deduplicated, deterministically ordered
// list of filters, each requiring a document field to match a value
// (itself query_string syntax, so e.g. "application*" is a valid value).
// When a field occurs more than once, the last occurrence wins.
func ParseFieldFilters(values []string) ([]esquery.FieldFilter, error) {
	pairs, err := parseKeyValuePairs(values, "document-field-filter")
	if err != nil {
		return nil, err
	}

	filters := make([]esquery.FieldFilter, len(pairs))
	for i, p := range pairs {
		filters[i] = esquery.FieldFilter{Field: p[0], Value: p[1]}
	}
	return filters, nil
}
