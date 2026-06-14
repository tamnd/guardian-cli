package guardian_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/guardian-cli/guardian"
)

const mockSearchResponse = `{
  "response": {
    "status": "ok",
    "total": 1,
    "results": [
      {
        "id": "technology/2024/jan/01/ai-future",
        "webTitle": "The future of AI",
        "sectionId": "technology",
        "webPublicationDate": "2024-01-01T10:00:00Z",
        "webUrl": "https://www.theguardian.com/technology/2024/jan/01/ai-future",
        "type": "article",
        "fields": {
          "byline": "Jane Doe",
          "trailText": "A look at where AI is heading."
        }
      }
    ]
  }
}`

const mockSectionsResponse = `{
  "response": {
    "status": "ok",
    "total": 2,
    "results": [
      {
        "id": "technology",
        "webTitle": "Technology",
        "webUrl": "https://www.theguardian.com/technology"
      },
      {
        "id": "science",
        "webTitle": "Science",
        "webUrl": "https://www.theguardian.com/science"
      }
    ]
  }
}`

const mockTagsResponse = `{
  "response": {
    "status": "ok",
    "total": 2,
    "results": [
      {
        "id": "technology/artificialintelligence",
        "webTitle": "Artificial intelligence (AI)",
        "sectionId": "technology",
        "type": "keyword"
      },
      {
        "id": "technology/computing",
        "webTitle": "Computing",
        "sectionId": "technology",
        "type": "keyword"
      }
    ]
  }
}`

func newTestClient(srv *httptest.Server) *guardian.Client {
	cfg := guardian.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	return guardian.NewClientWithConfig(cfg)
}

func TestSearchArticles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockSearchResponse))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	articles, err := c.SearchArticles(context.Background(), "AI", "", "", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("got %d articles, want 1", len(articles))
	}
	a := articles[0]
	if a.ID != "technology/2024/jan/01/ai-future" {
		t.Errorf("ID = %q", a.ID)
	}
	if a.Title != "The future of AI" {
		t.Errorf("Title = %q", a.Title)
	}
	if a.Section != "technology" {
		t.Errorf("Section = %q", a.Section)
	}
	if a.Author != "Jane Doe" {
		t.Errorf("Author = %q", a.Author)
	}
	if a.Trail != "A look at where AI is heading." {
		t.Errorf("Trail = %q", a.Trail)
	}
	if a.URL != "https://www.theguardian.com/technology/2024/jan/01/ai-future" {
		t.Errorf("URL = %q", a.URL)
	}
}

func TestSearchWithSection(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockSearchResponse))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.SearchArticles(context.Background(), "AI", "technology", "", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capturedURL, "section=technology") {
		t.Errorf("URL %q does not contain section=technology", capturedURL)
	}
}

func TestListSections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockSectionsResponse))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	sections, err := c.ListSections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	if sections[0].ID != "technology" {
		t.Errorf("sections[0].ID = %q, want technology", sections[0].ID)
	}
	if sections[0].Title != "Technology" {
		t.Errorf("sections[0].Title = %q, want Technology", sections[0].Title)
	}
	if sections[1].ID != "science" {
		t.Errorf("sections[1].ID = %q, want science", sections[1].ID)
	}
}

func TestListTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockTagsResponse))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	tags, err := c.ListTags(context.Background(), "technology", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Fatalf("got %d tags, want 2", len(tags))
	}
	if tags[0].ID != "technology/artificialintelligence" {
		t.Errorf("tags[0].ID = %q", tags[0].ID)
	}
	if tags[0].Type != "keyword" {
		t.Errorf("tags[0].Type = %q, want keyword", tags[0].Type)
	}
	if tags[0].Section != "technology" {
		t.Errorf("tags[0].Section = %q, want technology", tags[0].Section)
	}
}

func TestRetry503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockSearchResponse))
	}))
	defer srv.Close()

	cfg := guardian.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := guardian.NewClientWithConfig(cfg)

	start := time.Now()
	articles, err := c.SearchArticles(context.Background(), "test", "", "", "", 5)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if len(articles) == 0 {
		t.Error("got 0 articles after retries")
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if elapsed < 500*time.Millisecond {
		t.Errorf("retries did not back off: elapsed=%v", elapsed)
	}
}
