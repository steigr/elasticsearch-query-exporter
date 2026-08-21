// Package probe implements the /probe HTTP handler that bridges a single
// Elasticsearch query to a Prometheus scrape.
package probe

import (
	"sync"
	"time"
)

// AfterTimeStore tracks, per internal query ID, the "query after time" —
// the upper bound of the time range covered by the previous successful
// scrape of that query. It is the only state the exporter keeps between
// scrapes.
type AfterTimeStore struct {
	mu   sync.Mutex
	data map[string]time.Time
}

func NewAfterTimeStore() *AfterTimeStore {
	return &AfterTimeStore{data: make(map[string]time.Time)}
}

// Lookup returns the stored "query after time" for id and whether it was present.
func (s *AfterTimeStore) Lookup(id string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data[id]
	return t, ok
}

// Set stores t as the "query after time" for id.
func (s *AfterTimeStore) Set(id string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = t
}
