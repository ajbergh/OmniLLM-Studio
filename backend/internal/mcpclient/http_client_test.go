package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// ── helpers ───────────────────────────────────────────────────────────────

// newTestHTTPServer returns a test HTTP server that implements a minimal
// Streamable HTTP MCP endpoint returning JSON responses (no SSE).
func newTestHTTPServer(t *testing.T, tools []Tool) *httptest.Server {
	t.Helper()

	var sessionID string
	var nextToolID int64

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req rpcRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			// Notifications (no id) → 202 Accepted.
			if req.ID == 0 && req.Method != "" {
				w.WriteHeader(http.StatusAccepted)
				return
			}

			w.Header().Set("Content-Type", "application/json")

			switch req.Method {
			case "initialize":
				sessionID = "test-session-id-1"
				w.Header().Set("Mcp-Session-Id", sessionID)
				writeJSONRPCResult(w, req.ID, map[string]interface{}{
					"protocolVersion": ProtocolVersion,
					"serverInfo":      map[string]interface{}{"name": "test-server", "version": "1.0"},
					"capabilities":    map[string]interface{}{},
				})

			case "tools/list":
				toolList := make([]map[string]interface{}, 0, len(tools))
				for _, tool := range tools {
					entry := map[string]interface{}{
						"name":        tool.Name,
						"description": tool.Description,
						"inputSchema": json.RawMessage(`{"type":"object"}`),
					}
					if tool.Title != "" {
						entry["title"] = tool.Title
					}
					toolList = append(toolList, entry)
				}
				writeJSONRPCResult(w, req.ID, map[string]interface{}{
					"tools": toolList,
				})

			case "tools/call":
				nextToolID++
				params, _ := req.Params.(map[string]interface{})
				name, _ := params["name"].(string)
				writeJSONRPCResult(w, req.ID, map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": fmt.Sprintf("called %s id=%d", name, nextToolID)},
					},
					"isError": false,
				})

			default:
				writeJSONRPCError(w, req.ID, -32601, "method not found")
			}

		case http.MethodDelete:
			// Session termination.
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}

// newSSETestHTTPServer returns a test HTTP server that responds to requests
// with SSE streams instead of plain JSON, to exercise the SSE path.
func newSSETestHTTPServer(t *testing.T, tools []Tool) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Notifications → 202 Accepted.
		if req.ID == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		var resultObj interface{}
		switch req.Method {
		case "initialize":
			resultObj = map[string]interface{}{
				"protocolVersion": ProtocolVersion,
				"serverInfo":      map[string]interface{}{"name": "sse-server", "version": "1.0"},
				"capabilities":    map[string]interface{}{},
			}
		case "tools/list":
			toolList := make([]map[string]interface{}, 0, len(tools))
			for _, tool := range tools {
				toolList = append(toolList, map[string]interface{}{
					"name":        tool.Name,
					"description": tool.Description,
					"inputSchema": json.RawMessage(`{"type":"object"}`),
				})
			}
			resultObj = map[string]interface{}{"tools": toolList}
		case "tools/call":
			params, _ := req.Params.(map[string]interface{})
			name, _ := params["name"].(string)
			resultObj = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "sse result for " + name},
				},
			}
		default:
			// Write an error SSE event.
			errResp := rpcResponse{
				JSONRPC: "2.0",
				ID:      json.RawMessage(fmt.Sprintf("%d", req.ID)),
				Error:   &rpcError{Code: -32601, Message: "method not found"},
			}
			data, _ := json.Marshal(errResp)
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		// Emit a server notification first (should be skipped by client).
		notif := rpcResponse{
			JSONRPC: "2.0",
			Method:  "notifications/progress",
		}
		notifData, _ := json.Marshal(notif)
		fmt.Fprintf(w, "data: %s\n\n", notifData)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Emit the actual response.
		resp := rpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(fmt.Sprintf("%d", req.ID)),
		}
		resp.Result, _ = json.Marshal(resultObj)
		data, _ := json.Marshal(resp)
		fmt.Fprintf(w, "data: %s\n\n", data)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
}

func writeJSONRPCResult(w http.ResponseWriter, id int64, result interface{}) {
	data, _ := json.Marshal(result)
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  json.RawMessage(data),
	}
	json.NewEncoder(w).Encode(resp)
}

func writeJSONRPCError(w http.ResponseWriter, id int64, code int, message string) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]interface{}{"code": code, "message": message},
	}
	json.NewEncoder(w).Encode(resp)
}

func testMCPServer(url string) models.MCPServer {
	return models.MCPServer{
		ID:        "test-http-server",
		Name:      "test",
		Transport: "http",
		URL:       &url,
	}
}

// ── JSON transport tests ───────────────────────────────────────────────────

func TestHTTPClientJSONStartAndListTools(t *testing.T) {
	tools := []Tool{
		{Name: "echo", Description: "Echoes input"},
		{Name: "ping", Description: "Pings"},
	}
	srv := newTestHTTPServer(t, tools)
	defer srv.Close()

	client := NewHTTPClient(testMCPServer(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Stop(context.Background())

	discovered, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(discovered) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(discovered))
	}
	assertHasMCPTool(t, discovered, "echo")
	assertHasMCPTool(t, discovered, "ping")
}

func TestHTTPClientJSONCallTool(t *testing.T) {
	tools := []Tool{{Name: "greet", Description: "Greets"}}
	srv := newTestHTTPServer(t, tools)
	defer srv.Close()

	client := NewHTTPClient(testMCPServer(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Stop(context.Background())

	result, err := client.CallTool(ctx, "greet", json.RawMessage(`{"name":"world"}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got MCP error")
	}
	normalized := NormalizeToolResult(result)
	if !strings.Contains(normalized.Content, "called greet") {
		t.Fatalf("unexpected tool result content: %q", normalized.Content)
	}
}

func TestHTTPClientSessionIDPropagation(t *testing.T) {
	var receivedSessionID string
	sessionSet := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Verify session ID is sent on requests after initialization.
		if sessionSet {
			receivedSessionID = r.Header.Get("Mcp-Session-Id")
		}

		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}

		if req.ID == 0 {
			sessionSet = true
			w.WriteHeader(http.StatusAccepted)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "expected-session")
			writeJSONRPCResult(w, req.ID, map[string]interface{}{
				"protocolVersion": ProtocolVersion,
				"serverInfo":      map[string]interface{}{"name": "s", "version": "1"},
				"capabilities":    map[string]interface{}{},
			})
		case "tools/list":
			writeJSONRPCResult(w, req.ID, map[string]interface{}{"tools": []interface{}{}})
		}
	}))
	defer srv.Close()

	client := NewHTTPClient(testMCPServer(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Stop(context.Background())

	if _, err := client.ListTools(ctx); err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if receivedSessionID != "expected-session" {
		t.Fatalf("expected Mcp-Session-Id header %q, got %q", "expected-session", receivedSessionID)
	}
}

func TestHTTPClientCustomHeaders(t *testing.T) {
	var capturedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")

		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}

		if req.ID == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		writeJSONRPCResult(w, req.ID, map[string]interface{}{
			"protocolVersion": ProtocolVersion,
			"serverInfo":      map[string]interface{}{"name": "s", "version": "1"},
			"capabilities":    map[string]interface{}{},
		})
	}))
	defer srv.Close()

	server := testMCPServer(srv.URL)
	server.Headers = map[string]string{"Authorization": "Bearer secret-token"}
	client := NewHTTPClient(server)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer client.Stop(context.Background())

	if capturedAuth != "Bearer secret-token" {
		t.Fatalf("expected Authorization header %q, got %q", "Bearer secret-token", capturedAuth)
	}
}

func TestHTTPClientStartRequiresURL(t *testing.T) {
	server := models.MCPServer{
		ID:        "no-url",
		Name:      "no-url",
		Transport: "http",
		// URL intentionally nil
	}
	client := NewHTTPClient(server)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	if err == nil {
		t.Fatal("expected error when URL is missing, got nil")
	}
	if !strings.Contains(err.Error(), "url is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPClientHTTPErrorOnInitialize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewHTTPClient(testMCPServer(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Start(ctx)
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── SSE transport tests ───────────────────────────────────────────────────

func TestHTTPClientSSEStartAndListTools(t *testing.T) {
	tools := []Tool{
		{Name: "sse_tool_a", Description: "SSE tool A"},
		{Name: "sse_tool_b", Description: "SSE tool B"},
	}
	srv := newSSETestHTTPServer(t, tools)
	defer srv.Close()

	client := NewHTTPClient(testMCPServer(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start (SSE): %v", err)
	}
	defer client.Stop(context.Background())

	discovered, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools (SSE): %v", err)
	}
	if len(discovered) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(discovered))
	}
	assertHasMCPTool(t, discovered, "sse_tool_a")
	assertHasMCPTool(t, discovered, "sse_tool_b")
}

func TestHTTPClientSSECallTool(t *testing.T) {
	tools := []Tool{{Name: "compute", Description: "Computes"}}
	srv := newSSETestHTTPServer(t, tools)
	defer srv.Close()

	client := NewHTTPClient(testMCPServer(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start (SSE): %v", err)
	}
	defer client.Stop(context.Background())

	result, err := client.CallTool(ctx, "compute", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool (SSE): %v", err)
	}
	normalized := NormalizeToolResult(result)
	if !strings.Contains(normalized.Content, "sse result for compute") {
		t.Fatalf("unexpected content: %q", normalized.Content)
	}
}

// ── interface compliance ───────────────────────────────────────────────────

// TestHTTPClientImplementsMCPClient verifies *HTTPClient satisfies MCPClient
// at compile time.
func TestHTTPClientImplementsMCPClient(t *testing.T) {
	var _ MCPClient = (*HTTPClient)(nil)
}

// TestStdioClientImplementsMCPClient verifies *Client satisfies MCPClient.
func TestStdioClientImplementsMCPClient(t *testing.T) {
	var _ MCPClient = (*Client)(nil)
}
