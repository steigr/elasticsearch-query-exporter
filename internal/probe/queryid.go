package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// QueryID computes the internal query ID that identifies a distinct probe
// request across scrapes: the final Elasticsearch query string, the label
// field mapping, and the optional caller-supplied query_id, concatenated
// and hashed. Two scrapes only share state (the "query after time") when
// all three match.
func QueryID(esQueryString string, labelFieldMap []LabelField, queryIDParam string) string {
	var b strings.Builder
	b.WriteString(esQueryString)
	b.WriteByte('\x00')
	for _, m := range labelFieldMap {
		b.WriteString(m.Label)
		b.WriteByte('=')
		b.WriteString(m.Field)
		b.WriteByte(';')
	}
	b.WriteByte('\x00')
	b.WriteString(queryIDParam)

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
