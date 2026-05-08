package news

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Client is an HTTP client for the Actually Relevant News API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

// NewClient creates a new Actually Relevant API client.
func NewClient(cfg Config) *Client {
	return &Client{
		BaseURL:    cfg.BaseURL,
		UserAgent:  cfg.UserAgent,
		HTTPClient: &http.Client{Timeout: cfg.Timeout},
	}
}

// GetStories fetches a paginated list of news stories.
func (c *Client) GetStories(ctx context.Context, q StoryQuery) (*StoryPage, error) {
	u, err := url.Parse(c.BaseURL + "/stories")
	if err != nil {
		return nil, fmt.Errorf("parse stories URL: %w", err)
	}

	params := url.Values{}
	if q.Page > 0 {
		params.Set("page", strconv.Itoa(q.Page))
	}
	if q.PageSize > 0 {
		params.Set("pageSize", strconv.Itoa(q.PageSize))
	}
	if q.IssueSlug != "" {
		params.Set("issueSlug", q.IssueSlug)
	}
	if q.Search != "" {
		params.Set("search", q.Search)
	}
	if len(q.EmotionTags) > 0 {
		params.Set("emotionTags", strings.Join(q.EmotionTags, ","))
	}

	u.RawQuery = params.Encode()

	var page StoryPage
	if err := c.doRequest(ctx, u.String(), &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetStory fetches a single story by slug.
func (c *Client) GetStory(ctx context.Context, slug string) (*Story, error) {
	u := c.BaseURL + "/stories/" + url.PathEscape(slug)
	var story Story
	if err := c.doRequest(ctx, u, &story); err != nil {
		return nil, err
	}
	return &story, nil
}

// GetRelated fetches stories related to a given story slug.
func (c *Client) GetRelated(ctx context.Context, slug string, limit int) ([]Story, error) {
	u, err := url.Parse(c.BaseURL + "/stories/" + url.PathEscape(slug) + "/related")
	if err != nil {
		return nil, fmt.Errorf("parse related URL: %w", err)
	}
	if limit > 0 {
		u.RawQuery = url.Values{"limit": {strconv.Itoa(limit)}}.Encode()
	}

	var stories []Story
	if err := c.doRequest(ctx, u.String(), &stories); err != nil {
		return nil, err
	}
	return stories, nil
}

// GetCluster fetches the cluster of stories around a given story slug.
func (c *Client) GetCluster(ctx context.Context, slug string) (*ClusterResponse, error) {
	u := c.BaseURL + "/stories/" + url.PathEscape(slug) + "/cluster"
	var resp ClusterResponse
	if err := c.doRequest(ctx, u, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetIssues fetches the list of available issue areas.
func (c *Client) GetIssues(ctx context.Context) ([]Issue, error) {
	u := c.BaseURL + "/issues"
	var issues []Issue
	if err := c.doRequest(ctx, u, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

// doRequest performs an HTTP GET request and decodes the JSON response.
func (c *Client) doRequest(ctx context.Context, urlStr string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// APIError represents a non-2xx response from the API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API returned %d: %s", e.StatusCode, e.Body)
}
