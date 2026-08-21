// Package esquery executes the Elasticsearch search behind a single probe
// and extracts the document fields the caller asked for.
package esquery

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	Username   string
	Password   string
	HTTPClient *http.Client
}

// Option configures a Client returned by NewClient.
type Option func(*Client) error

// WithBasicAuth sends username/password as HTTP basic auth on every request.
func WithBasicAuth(username, password string) Option {
	return func(c *Client) error {
		c.Username = username
		c.Password = password
		return nil
	}
}

// WithCACertFile trusts the PEM-encoded CA certificate at path in addition
// to the system pool, for clusters serving a certificate signed by a
// private CA (e.g. ECK's self-signed transport/HTTP CA).
func WithCACertFile(path string) Option {
	return func(c *Client) error {
		pem, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read CA certificate: %w", err)
		}

		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return fmt.Errorf("no certificates found in %s", path)
		}

		transport := transportOf(c)
		transport.TLSClientConfig.RootCAs = pool
		return nil
	}
}

// WithInsecureSkipVerify disables TLS certificate verification. Intended for
// local testing only.
func WithInsecureSkipVerify(skip bool) Option {
	return func(c *Client) error {
		transportOf(c).TLSClientConfig.InsecureSkipVerify = skip
		return nil
	}
}

// transportOf returns c's *http.Transport, creating one with an initialized
// TLSClientConfig if c.HTTPClient doesn't already have one.
func transportOf(c *Client) *http.Transport {
	transport, ok := c.HTTPClient.Transport.(*http.Transport)
	if !ok || transport == nil {
		transport = http.DefaultTransport.(*http.Transport).Clone()
	}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	c.HTTPClient.Transport = transport
	return transport
}

func NewClient(baseURL string, opts ...Option) (*Client, error) {
	c := &Client{BaseURL: baseURL, HTTPClient: &http.Client{}}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	return c, nil
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
	if c.Username != "" {
		httpReq.SetBasicAuth(c.Username, c.Password)
	}

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
