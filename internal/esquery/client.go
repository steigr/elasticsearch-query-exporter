// Package esquery executes the Elasticsearch search behind a single probe
// and extracts the document fields the caller asked for.
package esquery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PatternType selects how SearchString is matched against documents.
type PatternType string

const (
	// PatternQueryString matches SearchString as a Lucene query_string
	// query. This is the recommended default: it supports glob-style
	// wildcards (*, ?) and boolean syntax, and — unlike a regexp query —
	// can be satisfied from the inverted index without a full field scan,
	// so prefer it (or plain wildcard syntax) over PatternRegexp whenever
	// the match can be expressed that way.
	PatternQueryString PatternType = "query_string"

	// PatternRegexp matches SearchString as an Elasticsearch regexp query
	// against a single field. Regexp queries cannot use the inverted
	// index the way query_string can and are noticeably more expensive at
	// scale, so this is opt-in for cases query_string syntax can't express.
	PatternRegexp PatternType = "regexp"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTPClient: http.DefaultClient}
}

// SearchRequest describes one probe's Elasticsearch query.
type SearchRequest struct {
	IndexPattern string
	Pattern      string
	PatternType  PatternType
	Field        string // field the pattern is matched against; "*" for query_string means "all fields"
	TimeField    string
	After        time.Time // exclusive
	Before       time.Time // inclusive
	SourceFields []string  // document fields to retrieve
}

// String renders the request's Elasticsearch query as the JSON string sent
// in the request body. It is deterministic for a given SearchRequest and is
// what feeds probe.QueryID.
func (r SearchRequest) String() string {
	body, _ := json.Marshal(r.buildQuery())
	return string(body)
}

func (r SearchRequest) buildQuery() map[string]any {
	var patternClause map[string]any
	switch r.PatternType {
	case PatternRegexp:
		patternClause = map[string]any{
			"regexp": map[string]any{r.Field: r.Pattern},
		}
	default:
		patternClause = map[string]any{
			"query_string": map[string]any{
				"query": r.Pattern,
				"field": r.Field,
			},
		}
	}

	return map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []any{patternClause},
				"filter": []any{
					map[string]any{
						"range": map[string]any{
							r.TimeField: map[string]any{
								"gt":  r.After.Format(time.RFC3339Nano),
								"lte": r.Before.Format(time.RFC3339Nano),
							},
						},
					},
				},
			},
		},
		"_source": r.SourceFields,
	}
}

// Hit is a single matched document's requested source fields.
type Hit struct {
	Source map[string]any
}

// Search runs req against the cluster and returns every matching document's
// requested source fields.
func (c *Client) Search(ctx context.Context, req SearchRequest) ([]Hit, error) {
	url := fmt.Sprintf("%s/%s/_search", c.BaseURL, req.IndexPattern)

	body, err := json.Marshal(req.buildQuery())
	if err != nil {
		return nil, fmt.Errorf("encode query: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("query elasticsearch: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read elasticsearch response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("elasticsearch returned %s: %s", resp.Status, respBody)
	}

	var parsed struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode elasticsearch response: %w", err)
	}

	hits := make([]Hit, 0, len(parsed.Hits.Hits))
	for _, h := range parsed.Hits.Hits {
		hits = append(hits, Hit{Source: h.Source})
	}
	return hits, nil
}
