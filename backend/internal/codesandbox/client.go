// Package codesandbox integrates an operator-configured external sandbox.
// OmniLLM-Studio never executes arbitrary model-generated code in the backend process.
package codesandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 2 << 20

type Client struct {
	baseURL string
	client  *http.Client
}

type ExecuteRequest struct {
	Language  string `json:"language"`
	Code      string `json:"code"`
	SessionID string `json:"session_id,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

type Artifact struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type,omitempty"`
	URL      string `json:"url,omitempty"`
	Bytes    int64  `json:"bytes,omitempty"`
}

type ExecuteResponse struct {
	SessionID string     `json:"session_id,omitempty"`
	Stdout    string     `json:"stdout,omitempty"`
	Stderr    string     `json:"stderr,omitempty"`
	ExitCode  int        `json:"exit_code"`
	Artifacts []Artifact `json:"artifacts,omitempty"`
}

func New(baseURL string) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("sandbox URL is required")
	}
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return nil, fmt.Errorf("sandbox URL must be an http(s) URL without userinfo")
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{Timeout: 65 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("sandbox redirects are disabled")
		}},
	}, nil
}

func (c *Client) Execute(ctx context.Context, request ExecuteRequest) (*ExecuteResponse, error) {
	body, _ := json.Marshal(request)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/execute", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("sandbox response exceeded %d bytes", maxResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sandbox returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out ExecuteResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode sandbox response: %w", err)
	}
	return &out, nil
}
