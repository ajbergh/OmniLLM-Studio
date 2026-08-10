package mcpclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
)

type protocolEra uint8

const (
	protocolEraUnknown protocolEra = iota
	protocolEraLegacy
	protocolEraModern
)

const (
	mcpHeaderMismatchError                     = -32020
	mcpMissingClientCapabilityError            = -32021
	mcpUnsupportedProtocolVersionError         = -32022
	maxJavaScriptSafeInteger           float64 = 9007199254740991
)

type mcpHeaderBinding struct {
	Name string
	Path []string
	Type string
}

func modernMeta() map[string]interface{} {
	return map[string]interface{}{
		"io.modelcontextprotocol/protocolVersion": ModernProtocolVersion,
		"io.modelcontextprotocol/clientInfo": map[string]interface{}{
			"name":    "omnillm-studio",
			"version": "0.2.0",
		},
		"io.modelcontextprotocol/clientCapabilities": map[string]interface{}{},
	}
}

func withModernRequestMeta(params interface{}) (interface{}, error) {
	out := map[string]interface{}{}
	if params != nil {
		value, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("modern MCP request params must be an object")
		}
		for key, item := range value {
			out[key] = item
		}
	}
	meta := map[string]interface{}{}
	if existing, ok := out["_meta"].(map[string]interface{}); ok {
		for key, item := range existing {
			meta[key] = item
		}
	}
	for key, item := range modernMeta() {
		meta[key] = item
	}
	out["_meta"] = meta
	return out, nil
}

func isRecognizedModernError(code int) bool {
	switch code {
	case mcpHeaderMismatchError, mcpMissingClientCapabilityError, mcpUnsupportedProtocolVersionError:
		return true
	default:
		return false
	}
}

func containsProtocolVersion(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// detectProtocolEra probes with the mandatory modern server/discover method.
// A valid modern response selects stateless 2026 behavior. Unrecognized legacy
// errors/responses fall back to the handshake-era client for compatibility.
func (c *HTTPClient) detectProtocolEra(ctx context.Context) error {
	c.mu.Lock()
	if c.era != protocolEraUnknown {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	params, _ := withModernRequestMeta(map[string]interface{}{})
	id := atomic.AddInt64(&c.nextID, 1)
	rpcReq := rpcRequest{JSONRPC: "2.0", ID: id, Method: "server/discover", Params: params}
	body, err := json.Marshal(rpcReq)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *c.server.URL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	if err := c.applyAuthHeaders(ctx, req); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", ModernProtocolVersion)
	req.Header.Set("Mcp-Method", "server/discover")

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("MCP protocol probe: %w", err)
	}
	defer resp.Body.Close()
	if err := c.handleOAuthScopeChallenge(resp); err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("MCP protocol probe returned HTTP %d", resp.StatusCode)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		rpcResp, parseErr := c.parseResponse(resp, id)
		if parseErr != nil {
			c.setProtocolEra(protocolEraLegacy)
			return nil
		}
		if rpcResp.Error != nil {
			if isRecognizedModernError(rpcResp.Error.Code) {
				return fmt.Errorf("MCP modern protocol probe: %w", rpcResp.Error)
			}
			c.setProtocolEra(protocolEraLegacy)
			return nil
		}
		var result discoverResult
		if err := json.Unmarshal(rpcResp.Result, &result); err != nil || len(result.SupportedVersions) == 0 {
			c.setProtocolEra(protocolEraLegacy)
			return nil
		}
		if !containsProtocolVersion(result.SupportedVersions, ModernProtocolVersion) {
			return fmt.Errorf("MCP server is modern but does not support %s (supports %s)", ModernProtocolVersion, strings.Join(result.SupportedVersions, ", "))
		}
		c.setProtocolEra(protocolEraModern)
		return nil
	}

	limited, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if readErr != nil {
		return fmt.Errorf("MCP protocol probe HTTP %d: %w", resp.StatusCode, readErr)
	}
	var rpcResp rpcResponse
	if json.Unmarshal(limited, &rpcResp) == nil && rpcResp.Error != nil && isRecognizedModernError(rpcResp.Error.Code) {
		return fmt.Errorf("MCP modern protocol probe: %w", rpcResp.Error)
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		c.setProtocolEra(protocolEraLegacy)
		return nil
	}
	return fmt.Errorf("MCP protocol probe returned HTTP %d", resp.StatusCode)
}

func (c *HTTPClient) setProtocolEra(era protocolEra) {
	c.mu.Lock()
	c.era = era
	if era == protocolEraModern {
		c.sessionID = ""
	}
	c.mu.Unlock()
}

func (c *HTTPClient) applyModernHeaders(req *http.Request, method string, params interface{}) error {
	req.Header.Set("MCP-Protocol-Version", ModernProtocolVersion)
	req.Header.Set("Mcp-Method", encodeMCPHeaderValue(method))
	paramMap, _ := params.(map[string]interface{})
	if method == "tools/call" || method == "resources/read" || method == "prompts/get" {
		nameKey := "name"
		if method == "resources/read" {
			nameKey = "uri"
		}
		name, _ := paramMap[nameKey].(string)
		if name == "" {
			return fmt.Errorf("%s requires %s for Mcp-Name", method, nameKey)
		}
		req.Header.Set("Mcp-Name", encodeMCPHeaderValue(name))
	}
	if method != "tools/call" {
		return nil
	}
	toolName, _ := paramMap["name"].(string)
	arguments, _ := paramMap["arguments"].(map[string]interface{})
	for _, binding := range c.toolHeaderBindings[toolName] {
		value, ok := nestedValue(arguments, binding.Path)
		if !ok {
			continue
		}
		formatted, err := formatMCPHeaderPrimitive(value, binding.Type)
		if err != nil {
			return fmt.Errorf("tool %q parameter %s: %w", toolName, strings.Join(binding.Path, "."), err)
		}
		req.Header.Set("Mcp-Param-"+binding.Name, encodeMCPHeaderValue(formatted))
	}
	return nil
}

func (c *HTTPClient) filterModernTools(tools []Tool) []Tool {
	filtered := make([]Tool, 0, len(tools))
	bindings := make(map[string][]mcpHeaderBinding, len(tools))
	for _, tool := range tools {
		toolBindings, err := parseMCPHeaderBindings(tool.InputSchema)
		if err != nil {
			log.Printf("[mcp] excluding modern HTTP tool %q: invalid x-mcp-header: %v", tool.Name, err)
			continue
		}
		bindings[tool.Name] = toolBindings
		filtered = append(filtered, tool)
	}
	c.toolHeaderBindings = bindings
	return filtered
}

func parseMCPHeaderBindings(schema json.RawMessage) ([]mcpHeaderBinding, error) {
	if len(schema) == 0 {
		return nil, nil
	}
	var root map[string]interface{}
	if err := json.Unmarshal(schema, &root); err != nil {
		return nil, fmt.Errorf("decode input schema: %w", err)
	}
	seen := map[string]bool{}
	var out []mcpHeaderBinding
	var walk func(map[string]interface{}, []string) error
	walk = func(node map[string]interface{}, path []string) error {
		if rawName, exists := node["x-mcp-header"]; exists {
			name, ok := rawName.(string)
			if !ok || name == "" || !validMCPHeaderToken(name) {
				return fmt.Errorf("header name must be a non-empty HTTP token")
			}
			if len(path) == 0 {
				return fmt.Errorf("header annotation must be attached to a parameter property")
			}
			typ, _ := node["type"].(string)
			if typ != "string" && typ != "integer" && typ != "boolean" {
				return fmt.Errorf("header %q must annotate string, integer, or boolean parameter", name)
			}
			key := strings.ToLower(name)
			if seen[key] {
				return fmt.Errorf("duplicate header name %q", name)
			}
			seen[key] = true
			out = append(out, mcpHeaderBinding{Name: name, Path: append([]string{}, path...), Type: typ})
		}
		if properties, ok := node["properties"].(map[string]interface{}); ok {
			for key, rawChild := range properties {
				if child, ok := rawChild.(map[string]interface{}); ok {
					if err := walk(child, append(path, key)); err != nil {
						return err
					}
				}
			}
		}
		for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
			if branches, ok := node[keyword].([]interface{}); ok {
				for _, rawBranch := range branches {
					if branch, ok := rawBranch.(map[string]interface{}); ok {
						if err := walk(branch, path); err != nil {
							return err
						}
					}
				}
			}
		}
		return nil
	}
	if err := walk(root, nil); err != nil {
		return nil, err
	}
	return out, nil
}

func validMCPHeaderToken(value string) bool {
	if value == "" {
		return false
	}
	const punctuation = "!#$%&'*+-.^_`|~"
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune(punctuation, r) {
			continue
		}
		return false
	}
	return true
}

func nestedValue(root map[string]interface{}, path []string) (interface{}, bool) {
	var current interface{} = root
	for _, segment := range path {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = object[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func formatMCPHeaderPrimitive(value interface{}, typ string) (string, error) {
	switch typ {
	case "string":
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("expected string")
		}
		return text, nil
	case "boolean":
		boolean, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("expected boolean")
		}
		return strconv.FormatBool(boolean), nil
	case "integer":
		switch number := value.(type) {
		case float64:
			if math.Trunc(number) != number || math.Abs(number) > maxJavaScriptSafeInteger {
				return "", fmt.Errorf("integer is outside the safe range")
			}
			return strconv.FormatInt(int64(number), 10), nil
		case json.Number:
			integer, err := number.Int64()
			if err != nil || math.Abs(float64(integer)) > maxJavaScriptSafeInteger {
				return "", fmt.Errorf("integer is outside the safe range")
			}
			return strconv.FormatInt(integer, 10), nil
		case int:
			if math.Abs(float64(number)) > maxJavaScriptSafeInteger {
				return "", fmt.Errorf("integer is outside the safe range")
			}
			return strconv.Itoa(number), nil
		case int64:
			if math.Abs(float64(number)) > maxJavaScriptSafeInteger {
				return "", fmt.Errorf("integer is outside the safe range")
			}
			return strconv.FormatInt(number, 10), nil
		default:
			return "", fmt.Errorf("expected integer")
		}
	default:
		return "", fmt.Errorf("unsupported header parameter type %q", typ)
	}
}

func encodeMCPHeaderValue(value string) string {
	unsafe := false
	if strings.HasPrefix(value, " ") || strings.HasPrefix(value, "\t") || strings.HasSuffix(value, " ") || strings.HasSuffix(value, "\t") {
		unsafe = true
	}
	if strings.HasPrefix(value, "=?base64?") && strings.HasSuffix(value, "?=") {
		unsafe = true
	}
	for _, b := range []byte(value) {
		if b < 0x20 || b > 0x7e || b == 0x7f {
			unsafe = true
			break
		}
	}
	if !unsafe {
		return value
	}
	return "=?base64?" + base64.StdEncoding.EncodeToString([]byte(value)) + "?="
}
