package probe

import (
	"fmt"
	"sort"
	"strings"
)

// parseKeyValuePairs parses repeated "key=value" query parameter values
// into a deduplicated, deterministically (key-sorted) ordered map. When a
// key occurs more than once, the last occurrence wins. paramName is used
// only to name the parameter in error messages.
func parseKeyValuePairs(values []string, paramName string) ([][2]string, error) {
	byKey := make(map[string]string, len(values))
	for _, v := range values {
		key, value, ok := strings.Cut(v, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid %s %q: expected format key=value", paramName, v)
		}
		byKey[key] = value // last occurrence wins
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([][2]string, len(keys))
	for i, key := range keys {
		pairs[i] = [2]string{key, byKey[key]}
	}
	return pairs, nil
}
