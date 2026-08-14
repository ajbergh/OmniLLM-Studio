package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxRuntimeResponseBytes = 4 << 20

// HTTPRuntime implements Runtime over the authenticated sandbox protocol v2.
// It is suitable for a separately isolated local/remote worker. Plain HTTP is
// accepted only for loopback endpoints; non-loopback workers require HTTPS.
type HTTPRuntime struct {
	baseURL      string
	token        string
	client       *http.Client
	capabilities RuntimeCapabilities
}

// NewHTTPRuntime validates the endpoint, requires an application-owned bearer
// token, and performs a bounded capabilities handshake before the runtime can be
// selected by Broker.
func NewHTTPRuntime(ctx context.Context, baseURL, token string) (*HTTPRuntime, error) {
	baseURL = strings.TrimSpace(baseURL)
	token = strings.TrimSpace(token)
	if baseURL == "" {
		return nil, fmt.Errorf("sandbox runtime URL is required")
	}
	if token == "" {
		return nil, fmt.Errorf("sandbox runtime token is required")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("sandbox runtime URL must be an http(s) URL without userinfo")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("sandbox runtime URL must not include query or fragment")
	}
	if u.Scheme == "http" && !isLoopbackHostname(u.Hostname()) {
		return nil, fmt.Errorf("non-loopback sandbox runtime requires https")
	}

	runtime := &HTTPRuntime{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client: &http.Client{
			Timeout: 65 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return fmt.Errorf("sandbox runtime redirects are disabled")
			},
		},
	}
	if err := runtime.doJSON(ctx, http.MethodGet, "/v2/capabilities", nil, &runtime.capabilities); err != nil {
		return nil, fmt.Errorf("sandbox capabilities handshake: %w", err)
	}
	if strings.TrimSpace(runtime.capabilities.Name) == "" {
		return nil, fmt.Errorf("sandbox runtime capabilities missing name")
	}
	return runtime, nil
}

// Capabilities reports the values authenticated during the runtime handshake.
func (r *HTTPRuntime) Capabilities() RuntimeCapabilities { return r.capabilities }

// Create allocates a runtime session for one Broker-owned sandbox.
func (r *HTTPRuntime) Create(ctx context.Context, request RuntimeCreateRequest) (string, error) {
	var response struct {
		RuntimeID string `json:"runtime_id"`
	}
	if err := r.doJSON(ctx, http.MethodPost, "/v2/sandboxes", request, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.RuntimeID) == "" {
		return "", fmt.Errorf("sandbox runtime returned empty runtime_id")
	}
	return response.RuntimeID, nil
}

// Exec executes one command inside a runtime-owned session.
func (r *HTTPRuntime) Exec(ctx context.Context, runtimeID string, request ExecRequest) (*ExecResult, error) {
	if request.ExecutionID != "" {
		if err := validateExecutionID(request.ExecutionID); err != nil {
			return nil, err
		}
	}
	var response ExecResult
	path := "/v2/sandboxes/" + url.PathEscape(runtimeID) + "/exec"
	if err := r.doJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		return nil, err
	}
	if request.ExecutionID != "" && response.ExecutionID != request.ExecutionID {
		return nil, fmt.Errorf("sandbox runtime returned mismatched execution id")
	}
	return &response, nil
}

// Cancel requests cancellation of one runtime execution.
func (r *HTTPRuntime) Cancel(ctx context.Context, runtimeID, executionID string) error {
	if err := validateExecutionID(executionID); err != nil {
		return err
	}
	path := "/v2/sandboxes/" + url.PathEscape(runtimeID) + "/cancel"
	return r.doJSON(ctx, http.MethodPost, path, map[string]string{"execution_id": executionID}, nil)
}

// Status reads runtime state for one session.
func (r *HTTPRuntime) Status(ctx context.Context, runtimeID string) (*Status, error) {
	var response Status
	path := "/v2/sandboxes/" + url.PathEscape(runtimeID) + "/status"
	if err := r.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// Destroy tears down one runtime session.
func (r *HTTPRuntime) Destroy(ctx context.Context, runtimeID string) error {
	path := "/v2/sandboxes/" + url.PathEscape(runtimeID)
	return r.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

func (r *HTTPRuntime) doJSON(ctx context.Context, method, path string, input, output interface{}) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode sandbox request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Accept", "application/json")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRuntimeResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxRuntimeResponseBytes {
		return fmt.Errorf("sandbox runtime response exceeded %d bytes", maxRuntimeResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sandbox runtime returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if output == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode sandbox runtime response: %w", err)
	}
	return nil
}

func isLoopbackHostname(host string) bool {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
