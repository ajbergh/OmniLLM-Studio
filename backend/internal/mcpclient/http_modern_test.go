package mcpclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestHTTPClientModern20260728UsesStatelessMetadataAndHeaders(t *testing.T) {
	var mu sync.Mutex
	initializeCalls := 0
	deleteCalls := 0
	toolCallSeen := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			mu.Lock()
			deleteCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Method == "initialize" {
			mu.Lock()
			initializeCalls++
			mu.Unlock()
			t.Fatal("modern HTTP server received legacy initialize")
		}
		if got := r.Header.Get("MCP-Protocol-Version"); got != ModernProtocolVersion {
			t.Fatalf("protocol header = %q", got)
		}
		if got := r.Header.Get("Mcp-Method"); got != req.Method {
			t.Fatalf("method header = %q, body = %q", got, req.Method)
		}
		if got := r.Header.Get("Mcp-Session-Id"); got != "" {
			t.Fatalf("modern request carried session ID %q", got)
		}
		params, ok := req.Params.(map[string]interface{})
		if !ok {
			t.Fatalf("params = %#v", req.Params)
		}
		meta, ok := params["_meta"].(map[string]interface{})
		if !ok || meta["io.modelcontextprotocol/protocolVersion"] != ModernProtocolVersion {
			t.Fatalf("modern metadata missing: %#v", params["_meta"])
		}

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "server/discover":
			writeJSONRPCResult(w, req.ID, map[string]interface{}{
				"resultType":        "complete",
				"supportedVersions": []string{ModernProtocolVersion},
				"capabilities":      map[string]interface{}{"tools": map[string]interface{}{}},
			})
		case "tools/list":
			writeJSONRPCResult(w, req.ID, map[string]interface{}{
				"resultType": "complete",
				"tools": []map[string]interface{}{{
					"name": "route",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"region": map[string]interface{}{"type": "string", "x-mcp-header": "Region"},
							"note":   map[string]interface{}{"type": "string", "x-mcp-header": "Note"},
						},
					},
				}},
			})
		case "tools/call":
			if got := r.Header.Get("Mcp-Name"); got != "route" {
				t.Fatalf("Mcp-Name = %q", got)
			}
			if got := r.Header.Get("Mcp-Param-Region"); got != "us-east1" {
				t.Fatalf("region header = %q", got)
			}
			wantNote := "=?base64?" + base64.StdEncoding.EncodeToString([]byte(" café")) + "?="
			if got := r.Header.Get("Mcp-Param-Note"); got != wantNote {
				t.Fatalf("note header = %q, want %q", got, wantNote)
			}
			mu.Lock()
			toolCallSeen = true
			mu.Unlock()
			writeJSONRPCResult(w, req.ID, map[string]interface{}{
				"resultType": "complete",
				"content":    []map[string]interface{}{{"type": "text", "text": "ok"}},
			})
		default:
			writeJSONRPCError(w, req.ID, -32601, "method not found")
		}
	}))
	defer server.Close()

	client := NewHTTPClient(testMCPServer(server.URL))
	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	tools, err := client.ListTools(ctx)
	if err != nil || len(tools) != 1 || tools[0].Name != "route" {
		t.Fatalf("tools = %#v, err=%v", tools, err)
	}
	result, err := client.CallTool(ctx, "route", json.RawMessage(`{"region":"us-east1","note":" café"}`))
	if err != nil || result == nil {
		t.Fatalf("CallTool: result=%#v err=%v", result, err)
	}
	if err := client.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if initializeCalls != 0 || deleteCalls != 0 || !toolCallSeen {
		t.Fatalf("modern lifecycle initialize=%d delete=%d toolCall=%v", initializeCalls, deleteCalls, toolCallSeen)
	}
}

func TestHTTPClientModernFiltersInvalidHeaderAnnotatedTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "server/discover":
			writeJSONRPCResult(w, req.ID, map[string]interface{}{"supportedVersions": []string{ModernProtocolVersion}})
		case "tools/list":
			writeJSONRPCResult(w, req.ID, map[string]interface{}{"tools": []map[string]interface{}{
				{"name": "good", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"x": map[string]interface{}{"type": "string", "x-mcp-header": "Tenant"}}}},
				{"name": "bad", "inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"x": map[string]interface{}{"type": "string", "x-mcp-header": "Bad Header"}}}},
			}})
		default:
			writeJSONRPCError(w, req.ID, -32601, "method not found")
		}
	}))
	defer server.Close()
	client := NewHTTPClient(testMCPServer(server.URL))
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "good" {
		t.Fatalf("filtered tools = %#v", tools)
	}
}

func TestHTTPClientFallsBackToLegacyInitialization(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		methods = append(methods, req.Method)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "server/discover":
			writeJSONRPCError(w, req.ID, -32601, "method not found")
		case "initialize":
			writeJSONRPCResult(w, req.ID, map[string]interface{}{"protocolVersion": LegacyProtocolVersion, "serverInfo": map[string]interface{}{"name": "legacy", "version": "1"}})
		default:
			if req.ID == 0 {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			writeJSONRPCResult(w, req.ID, map[string]interface{}{})
		}
	}))
	defer server.Close()
	client := NewHTTPClient(testMCPServer(server.URL))
	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(methods) < 3 || methods[0] != "server/discover" || methods[1] != "initialize" || methods[2] != "notifications/initialized" {
		t.Fatalf("legacy method sequence = %v", methods)
	}
}

func TestEncodeMCPHeaderValue(t *testing.T) {
	if got := encodeMCPHeaderValue("plain-value"); got != "plain-value" {
		t.Fatalf("plain header changed: %q", got)
	}
	for _, value := range []string{" leading", "café", "=?base64?ambiguous?="} {
		if got := encodeMCPHeaderValue(value); !strings.HasPrefix(got, "=?base64?") || !strings.HasSuffix(got, "?=") {
			t.Fatalf("unsafe value %q was not encoded: %q", value, got)
		}
	}
}
