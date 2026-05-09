package mcpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// HTTPClient implements MCPClient over the MCP 2025-06-18 Streamable HTTP transport.
//
// Every JSON-RPC message is a fresh HTTP POST to the server's single MCP endpoint.
// The server MAY respond with Content-Type: application/json (single JSON-RPC object)
// or Content-Type: text/event-stream (SSE stream) — both are handled transparently.
// Notifications (no id) expect a 202 Accepted response with an empty body.
// An optional Mcp-Session-Id returned by the server during initialization is
// attached to all subsequent requests.
type HTTPClient struct {
	server  models.MCPServer
	httpCli *http.Client

	mu        sync.Mutex
	sessionID string
	stopped   bool

	nextID int64
}

// NewHTTPClient creates an HTTPClient for a configured MCP server.
func NewHTTPClient(server models.MCPServer) *HTTPClient {
	return &HTTPClient{
		server: server,
		httpCli: &http.Client{
			Timeout: time.Duration(defaultRequestTimeout) * time.Second,
		},
	}
}

// Start connects to the MCP HTTP endpoint and performs MCP initialization.
func (c *HTTPClient) Start(ctx context.Context) error {
	if c.server.URL == nil || strings.TrimSpace(*c.server.URL) == "" {
		return fmt.Errorf("url is required for http MCP server")
	}
	return c.initialize(ctx)
}

// Stop terminates the session by issuing an HTTP DELETE (best-effort).
func (c *HTTPClient) Stop(ctx context.Context) error {
	c.mu.Lock()
	sessionID := c.sessionID
	c.stopped = true
	c.mu.Unlock()

	if sessionID == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, *c.server.URL, nil)
	if err != nil {
		return nil // best-effort
	}
	req.Header.Set("Mcp-Session-Id", sessionID)
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	c.applyCustomHeaders(req)

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil // best-effort
	}
	resp.Body.Close()
	return nil
}

// ListTools queries the server for all tools, following pagination cursors.
func (c *HTTPClient) ListTools(ctx context.Context) ([]Tool, error) {
	var tools []Tool
	cursor := ""

	for {
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}

		var result listToolsResult
		if err := c.request(ctx, "tools/list", params, &result); err != nil {
			return nil, err
		}
		tools = append(tools, result.Tools...)
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	return tools, nil
}

// CallTool invokes a named tool on the server.
func (c *HTTPClient) CallTool(ctx context.Context, name string, arguments json.RawMessage) (*CallToolResult, error) {
	var args map[string]interface{}
	if len(bytes.TrimSpace(arguments)) == 0 {
		args = map[string]interface{}{}
	} else if err := json.Unmarshal(arguments, &args); err != nil {
		return nil, fmt.Errorf("arguments must be a JSON object: %w", err)
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	var result CallToolResult
	if err := c.request(ctx, "tools/call", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── internal helpers ──────────────────────────────────────────────────────

func (c *HTTPClient) initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo": implementation{
			Name:    "omnillm-studio",
			Version: "0.2.0",
		},
	}

	id := atomic.AddInt64(&c.nextID, 1)
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "initialize",
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal initialize request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, *c.server.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build initialize request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	c.applyCustomHeaders(httpReq)

	httpResp, err := c.httpCli.Do(httpReq)
	if err != nil {
		return fmt.Errorf("initialize MCP server: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1024))
		return fmt.Errorf("initialize MCP server: HTTP %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Save session ID if the server issued one.
	if sid := httpResp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.mu.Lock()
		c.sessionID = sid
		c.mu.Unlock()
	}

	// Parse response — either plain JSON or an SSE stream.
	rpcResp, err := c.parseResponse(httpResp, id)
	if err != nil {
		return fmt.Errorf("initialize MCP server: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("initialize MCP server: %w", rpcResp.Error)
	}

	var result initializeResult
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return fmt.Errorf("initialize MCP server: decode result: %w", err)
	}
	if result.ProtocolVersion == "" {
		return fmt.Errorf("initialize MCP server: server did not return protocol version")
	}

	// Notify the server that initialization is complete.
	return c.notify(ctx, "notifications/initialized", nil)
}

// request sends a JSON-RPC request and returns the decoded result into out.
func (c *HTTPClient) request(ctx context.Context, method string, params interface{}, out interface{}) error {
	id := atomic.AddInt64(&c.nextID, 1)
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal %s request: %w", method, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, *c.server.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s request: %w", method, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	c.applySessionHeader(httpReq)
	c.applyCustomHeaders(httpReq)

	httpResp, err := c.httpCli.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		// Session expired per the spec — caller can re-initialize.
		return fmt.Errorf("%s: MCP session expired (HTTP 404)", method)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		snip, _ := io.ReadAll(io.LimitReader(httpResp.Body, 512))
		return fmt.Errorf("%s: HTTP %d: %s", method, httpResp.StatusCode, strings.TrimSpace(string(snip)))
	}

	rpcResp, err := c.parseResponse(httpResp, id)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	if rpcResp.Error != nil {
		return rpcResp.Error
	}
	if out != nil && len(rpcResp.Result) > 0 {
		if err := json.Unmarshal(rpcResp.Result, out); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
	}
	return nil
}

// notify sends a JSON-RPC notification (no id) and expects 202 Accepted.
func (c *HTTPClient) notify(ctx context.Context, method string, params interface{}) error {
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal %s notification: %w", method, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, *c.server.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s notification: %w", method, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	c.applySessionHeader(httpReq)
	c.applyCustomHeaders(httpReq)

	httpResp, err := c.httpCli.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	httpResp.Body.Close()

	if httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("%s: unexpected HTTP %d", method, httpResp.StatusCode)
}

// parseResponse reads the HTTP response body as either plain JSON or an SSE
// stream, returning the JSON-RPC response whose id matches the request id.
func (c *HTTPClient) parseResponse(httpResp *http.Response, id int64) (rpcResponse, error) {
	ct := httpResp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return readSSEResponse(httpResp.Body, id)
	}
	return readJSONResponse(httpResp.Body)
}

func (c *HTTPClient) applySessionHeader(req *http.Request) {
	c.mu.Lock()
	sid := c.sessionID
	c.mu.Unlock()
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
	}
}

func (c *HTTPClient) applyCustomHeaders(req *http.Request) {
	for k, v := range c.server.Headers {
		req.Header.Set(k, v)
	}
}

// ── SSE / JSON helpers ────────────────────────────────────────────────────

// readJSONResponse decodes a single JSON-RPC response from the body.
func readJSONResponse(body io.Reader) (rpcResponse, error) {
	var resp rpcResponse
	if err := json.NewDecoder(io.LimitReader(body, int64(maxRPCMessageBytes))).Decode(&resp); err != nil {
		return resp, fmt.Errorf("decode JSON response: %w", err)
	}
	return resp, nil
}

// readSSEResponse reads SSE events from body until it finds the JSON-RPC
// response matching the given request id.  The server may send server-initiated
// notifications or requests before sending the matching response; those are
// skipped transparently.
func readSSEResponse(body io.Reader, id int64) (rpcResponse, error) {
	scanner := bufio.NewScanner(io.LimitReader(body, int64(maxRPCMessageBytes)))
	idStr := fmt.Sprintf("%d", id)
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Blank line: dispatch the accumulated event data.
			if len(dataLines) > 0 {
				data := strings.Join(dataLines, "\n")
				dataLines = dataLines[:0]

				var resp rpcResponse
				if err := json.Unmarshal([]byte(data), &resp); err != nil {
					continue // skip non-JSON events
				}
				// Match our request ID (stored as a raw JSON number string).
				if string(resp.ID) == idStr {
					return resp, nil
				}
				// Server-initiated request or notification — skip and keep reading.
			}
			continue
		}

		if after, ok := strings.CutPrefix(line, "data:"); ok {
			// Trim the single optional leading space defined by the SSE spec.
			if len(after) > 0 && after[0] == ' ' {
				after = after[1:]
			}
			dataLines = append(dataLines, after)
		}
		// Ignore id:, event:, retry: SSE control fields.
	}

	if err := scanner.Err(); err != nil {
		return rpcResponse{}, fmt.Errorf("read SSE stream: %w", err)
	}
	return rpcResponse{}, fmt.Errorf("SSE stream closed before receiving response for request %d", id)
}
