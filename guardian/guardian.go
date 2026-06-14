// Package guardian is the library behind the guardian command line:
// the HTTP client, request shaping, and the typed data models for
// The Guardian newspaper API.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package guardian

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Host is the API host this client talks to.
const Host = "content.guardianapis.com"

// BaseURL is the root every request is built from.
const BaseURL = "https://content.guardianapis.com"

// Config holds all tunable knobs for the client.
type Config struct {
	BaseURL   string
	APIKey    string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults using the official free demo key.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		APIKey:    "test",
		UserAgent: "tamnd-guardian-cli/0.1 (tamnd87@gmail.com)",
		Rate:      200 * time.Millisecond,
		Retries:   3,
		Timeout:   15 * time.Second,
	}
}

// Client talks to The Guardian content API over HTTPS.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client with DefaultConfig applied.
func NewClient() *Client {
	cfg := DefaultConfig()
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// NewClientWithConfig returns a Client using the provided Config.
func NewClientWithConfig(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultConfig().Timeout
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// --- Output types ---

// Article is a single piece of content from The Guardian.
type Article struct {
	ID          string `json:"id" kit:"id"`
	Title       string `json:"title"`
	Section     string `json:"section"`
	PublishedAt string `json:"published_at"`
	Author      string `json:"author"`
	Trail       string `json:"trail"`
	URL         string `json:"url"`
}

// Section is a content section on The Guardian website.
type Section struct {
	ID    string `json:"id" kit:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Tag is a topic tag used to classify Guardian content.
type Tag struct {
	ID      string `json:"id" kit:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Section string `json:"section"`
}

// --- Wire types (internal JSON mapping) ---

type wireResponse struct {
	Response wireBody `json:"response"`
}

type wireBody struct {
	Status  string       `json:"status"`
	Total   int          `json:"total"`
	Results []wireResult `json:"results"`
}

type wireResult struct {
	ID                 string     `json:"id"`
	WebTitle           string     `json:"webTitle"`
	SectionID          string     `json:"sectionId"`
	WebPublicationDate string     `json:"webPublicationDate"`
	WebURL             string     `json:"webUrl"`
	Type               string     `json:"type"`
	Fields             wireFields `json:"fields"`
}

type wireFields struct {
	Byline    string `json:"byline"`
	TrailText string `json:"trailText"`
}

// --- Methods ---

// SearchArticles queries The Guardian content search API.
func (c *Client) SearchArticles(ctx context.Context, query, section, from, to string, limit int) ([]Article, error) {
	params := url.Values{}
	params.Set("api-key", c.cfg.APIKey)
	params.Set("show-fields", "trailText,byline")
	if query != "" {
		params.Set("q", query)
	}
	if section != "" {
		params.Set("section", section)
	}
	if from != "" {
		params.Set("from-date", from)
	}
	if to != "" {
		params.Set("to-date", to)
	}
	if limit > 0 {
		if limit > 200 {
			limit = 200
		}
		params.Set("page-size", strconv.Itoa(limit))
	}

	endpoint := c.cfg.BaseURL + "/search?" + params.Encode()
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var resp wireResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("guardian: decode search response: %w", err)
	}

	articles := make([]Article, 0, len(resp.Response.Results))
	for _, r := range resp.Response.Results {
		articles = append(articles, Article{
			ID:          r.ID,
			Title:       r.WebTitle,
			Section:     r.SectionID,
			PublishedAt: r.WebPublicationDate,
			Author:      r.Fields.Byline,
			Trail:       r.Fields.TrailText,
			URL:         r.WebURL,
		})
	}
	return articles, nil
}

// ListSections returns all content sections available on The Guardian.
func (c *Client) ListSections(ctx context.Context) ([]Section, error) {
	params := url.Values{}
	params.Set("api-key", c.cfg.APIKey)

	endpoint := c.cfg.BaseURL + "/sections?" + params.Encode()
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var resp wireResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("guardian: decode sections response: %w", err)
	}

	sections := make([]Section, 0, len(resp.Response.Results))
	for _, r := range resp.Response.Results {
		sections = append(sections, Section{
			ID:    r.ID,
			Title: r.WebTitle,
			URL:   r.WebURL,
		})
	}
	return sections, nil
}

// ListTags returns tags from The Guardian, optionally filtered by section.
func (c *Client) ListTags(ctx context.Context, section string, limit int) ([]Tag, error) {
	params := url.Values{}
	params.Set("api-key", c.cfg.APIKey)
	if section != "" {
		params.Set("section", section)
	}
	if limit > 0 {
		if limit > 200 {
			limit = 200
		}
		params.Set("page-size", strconv.Itoa(limit))
	}

	endpoint := c.cfg.BaseURL + "/tags?" + params.Encode()
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var resp wireResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("guardian: decode tags response: %w", err)
	}

	tags := make([]Tag, 0, len(resp.Response.Results))
	for _, r := range resp.Response.Results {
		tags = append(tags, Tag{
			ID:      r.ID,
			Type:    r.Type,
			Title:   r.WebTitle,
			Section: r.SectionID,
		})
	}
	return tags, nil
}

// --- Internal HTTP helpers ---

func (c *Client) get(ctx context.Context, endpoint string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, endpoint)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", endpoint, lastErr)
}

func (c *Client) do(ctx context.Context, endpoint string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
