// Package openapi turns configured OpenAPI 3 operations into governed OmniLLM tools.
package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

const maxResponseBytes = 1 << 20

var toolNameSanitizer = regexp.MustCompile(`[^a-z0-9_]+`)

type Manager struct {
	repo     *repository.OpenAPIServerRepo
	permRepo *repository.ToolPermissionRepo
	registry *tools.Registry
	mu       sync.Mutex
	byServer map[string][]string
	infos    map[string][]models.OpenAPIToolInfo
}

func NewManager(repo *repository.OpenAPIServerRepo, permRepo *repository.ToolPermissionRepo, registry *tools.Registry) *Manager {
	return &Manager{repo: repo, permRepo: permRepo, registry: registry, byServer: map[string][]string{}, infos: map[string][]models.OpenAPIToolInfo{}}
}

func (m *Manager) LoadEnabled(ctx context.Context) error {
	servers, err := m.repo.ListRuntimeEnabled()
	if err != nil {
		return err
	}
	for _, server := range servers {
		if _, err := m.register(ctx, server); err != nil {
			return fmt.Errorf("register OpenAPI server %s: %w", server.Name, err)
		}
	}
	return nil
}

func (m *Manager) Refresh(ctx context.Context, serverID string) ([]models.OpenAPIToolInfo, error) {
	m.Remove(serverID)
	server, err := m.repo.GetRuntime(serverID)
	if err != nil {
		return nil, err
	}
	if server == nil || !server.Enabled {
		return []models.OpenAPIToolInfo{}, nil
	}
	return m.register(ctx, *server)
}

func (m *Manager) Remove(serverID string) {
	m.mu.Lock()
	names := append([]string(nil), m.byServer[serverID]...)
	delete(m.byServer, serverID)
	delete(m.infos, serverID)
	m.mu.Unlock()
	for _, name := range names {
		m.registry.Remove(name)
	}
}

func (m *Manager) Tools(serverID string) []models.OpenAPIToolInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.OpenAPIToolInfo(nil), m.infos[serverID]...)
}

func (m *Manager) register(_ context.Context, server models.OpenAPIServerRuntime) ([]models.OpenAPIToolInfo, error) {
	if err := validateBaseURL(server.BaseURL, server.AllowPrivateNetwork); err != nil {
		return nil, err
	}
	operations, err := parseOperations(server)
	if err != nil {
		return nil, err
	}
	registered := make([]string, 0, len(operations))
	infos := make([]models.OpenAPIToolInfo, 0, len(operations))
	for _, op := range operations {
		adapter := &operationTool{server: server, operation: op}
		if err := m.registry.Register(adapter); err != nil {
			for _, name := range registered {
				m.registry.Remove(name)
			}
			return nil, err
		}
		registered = append(registered, op.ToolName)
		policy := "ask"
		if m.permRepo != nil {
			if existing, getErr := m.permRepo.Get(op.ToolName); getErr != nil {
				return nil, getErr
			} else if existing == nil {
				if upsertErr := m.permRepo.Upsert(op.ToolName, "ask"); upsertErr != nil {
					return nil, upsertErr
				}
			} else {
				policy = existing.Policy
			}
		}
		infos = append(infos, models.OpenAPIToolInfo{Name: op.ToolName, OperationID: op.OperationID, Method: op.Method, Path: op.Path, Description: op.Description, Policy: policy})
	}
	m.mu.Lock()
	m.byServer[server.ID] = registered
	m.infos[server.ID] = infos
	m.mu.Unlock()
	return infos, nil
}

type document struct {
	OpenAPI string                                `json:"openapi"`
	Paths   map[string]map[string]json.RawMessage `json:"paths"`
}
type parameter struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Description string          `json:"description"`
	Required    bool            `json:"required"`
	Schema      json.RawMessage `json:"schema"`
}
type requestBody struct {
	Required bool `json:"required"`
	Content  map[string]struct {
		Schema json.RawMessage `json:"schema"`
	} `json:"content"`
}
type operationSpec struct {
	OperationID string       `json:"operationId"`
	Summary     string       `json:"summary"`
	Description string       `json:"description"`
	Parameters  []parameter  `json:"parameters"`
	RequestBody *requestBody `json:"requestBody"`
}
type operation struct {
	ToolName, OperationID, Method, Path, Description string
	Parameters                                       []parameter
	BodySchema                                       json.RawMessage
	BodyRequired                                     bool
	InputSchema                                      json.RawMessage
}

func parseOperations(server models.OpenAPIServerRuntime) ([]operation, error) {
	var doc document
	if err := json.Unmarshal([]byte(server.SpecJSON), &doc); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI JSON: %w", err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		return nil, fmt.Errorf("only OpenAPI 3.x JSON is supported")
	}
	methods := map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true, "head": true, "options": true}
	var out []operation
	for path, entries := range doc.Paths {
		for method, raw := range entries {
			methodLower := strings.ToLower(method)
			if !methods[methodLower] {
				continue
			}
			var spec operationSpec
			if err := json.Unmarshal(raw, &spec); err != nil {
				return nil, fmt.Errorf("parse %s %s: %w", method, path, err)
			}
			if strings.TrimSpace(spec.OperationID) == "" {
				continue
			}
			description := strings.TrimSpace(spec.Description)
			if description == "" {
				description = strings.TrimSpace(spec.Summary)
			}
			if description == "" {
				description = methodLower + " " + path
			}
			shortID := server.ID
			if len(shortID) > 6 {
				shortID = shortID[:6]
			}
			toolName := sanitizeName("openapi_" + server.Name + "_" + spec.OperationID + "_" + shortID)
			properties := map[string]json.RawMessage{}
			required := []string{}
			supportedParams := []parameter{}
			for _, p := range spec.Parameters {
				if p.In != "path" && p.In != "query" {
					continue
				}
				schema := p.Schema
				if len(schema) == 0 {
					schema = json.RawMessage(`{"type":"string"}`)
				}
				properties[p.Name] = schema
				supportedParams = append(supportedParams, p)
				if p.Required || p.In == "path" {
					required = append(required, p.Name)
				}
			}
			var bodySchema json.RawMessage
			bodyRequired := false
			if spec.RequestBody != nil {
				if media, ok := spec.RequestBody.Content["application/json"]; ok && len(media.Schema) > 0 {
					bodySchema = media.Schema
					properties["body"] = media.Schema
					bodyRequired = spec.RequestBody.Required
					if bodyRequired {
						required = append(required, "body")
					}
				}
			}
			schemaObj := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
			if len(required) > 0 {
				schemaObj["required"] = required
			}
			input, _ := json.Marshal(schemaObj)
			out = append(out, operation{ToolName: toolName, OperationID: spec.OperationID, Method: strings.ToUpper(methodLower), Path: path, Description: description, Parameters: supportedParams, BodySchema: bodySchema, BodyRequired: bodyRequired, InputSchema: input})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ToolName < out[j].ToolName })
	if len(out) == 0 {
		return nil, fmt.Errorf("OpenAPI spec contains no operations with operationId")
	}
	return out, nil
}

func sanitizeName(value string) string {
	value = strings.ToLower(value)
	value = toolNameSanitizer.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

type operationTool struct {
	server    models.OpenAPIServerRuntime
	operation operation
}

func (t *operationTool) Definition() tools.ToolDefinition {
	readOnly := t.operation.Method == http.MethodGet || t.operation.Method == http.MethodHead
	return tools.ToolDefinition{Name: t.operation.ToolName, Description: t.operation.Description, Category: "openapi", Enabled: true, ReadOnly: readOnly, SideEffecting: !readOnly, Risk: func() tools.RiskLevel {
		if readOnly {
			return tools.RiskLow
		}
		return tools.RiskHigh
	}(), SupportsParallel: readOnly, RequiresNetwork: true, RequiresCredentials: t.server.APIKey != "", DefaultTimeoutMS: 30000, MaxResultBytes: maxResponseBytes, Parameters: t.operation.InputSchema}
}
func (t *operationTool) Validate(args json.RawMessage) error {
	var values map[string]json.RawMessage
	if err := json.Unmarshal(args, &values); err != nil {
		return err
	}
	for _, p := range t.operation.Parameters {
		if p.Required || p.In == "path" {
			if _, ok := values[p.Name]; !ok {
				return fmt.Errorf("%s is required", p.Name)
			}
		}
	}
	if t.operation.BodyRequired {
		if _, ok := values["body"]; !ok {
			return fmt.Errorf("body is required")
		}
	}
	return nil
}
func (t *operationTool) Execute(ctx context.Context, args json.RawMessage) (*tools.ToolResult, error) {
	scope := tools.InvocationScopeFromContext(ctx)
	if t.server.OwnerUserID != "" && scope.UserID != "" && t.server.OwnerUserID != scope.UserID {
		return nil, fmt.Errorf("OpenAPI tool is not available to this user")
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(args, &values); err != nil {
		return nil, err
	}
	base, err := url.Parse(t.server.BaseURL)
	if err != nil {
		return nil, err
	}
	path := t.operation.Path
	q := base.Query()
	for _, p := range t.operation.Parameters {
		raw, ok := values[p.Name]
		if !ok {
			continue
		}
		text, err := scalarText(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name, err)
		}
		if p.In == "path" {
			path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(text))
		} else if p.In == "query" {
			q.Set(p.Name, text)
		}
	}
	relative, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	target := base.ResolveReference(relative)
	target.RawQuery = q.Encode()
	if target.Host != base.Host {
		return nil, fmt.Errorf("operation target escaped configured host")
	}
	var body io.Reader
	if raw, ok := values["body"]; ok {
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, t.operation.Method, target.String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if t.server.APIKey != "" {
		value := t.server.APIKey
		if strings.TrimSpace(t.server.AuthPrefix) != "" {
			value = strings.TrimSpace(t.server.AuthPrefix) + " " + value
		}
		req.Header.Set(t.server.AuthHeader, value)
	}
	client := safeHTTPClient(t.server.AllowPrivateNetwork)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	truncated := len(data) > maxResponseBytes
	if truncated {
		data = data[:maxResponseBytes]
	}
	structured, _ := json.Marshal(map[string]any{"status": resp.StatusCode, "content_type": resp.Header.Get("Content-Type"), "body": string(data), "truncated": truncated})
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &tools.ToolResult{Content: fmt.Sprintf("OpenAPI request returned HTTP %d: %s", resp.StatusCode, string(data)), Structured: structured, IsError: true}, nil
	}
	return &tools.ToolResult{Content: string(data), Structured: structured, Metadata: map[string]interface{}{"truncated": truncated}}, nil
}
func scalarText(raw json.RawMessage) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	switch v := value.(type) {
	case string:
		return v, nil
	case float64, bool:
		return fmt.Sprint(v), nil
	default:
		return "", fmt.Errorf("must be a scalar value")
	}
}

func validateBaseURL(raw string, allowPrivate bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base_url must use http or https")
	}
	if u.User != nil {
		return fmt.Errorf("base_url userinfo is not allowed")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("base_url host is required")
	}
	if strings.EqualFold(host, "localhost") {
		if !allowPrivate {
			return fmt.Errorf("private or loopback base_url requires allow_private_network")
		}
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && blockedIP(ip) && !allowPrivate {
		return fmt.Errorf("private or loopback base_url requires allow_private_network")
	}
	return nil
}
func blockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
func safeHTTPClient(allowPrivate bool) *http.Client {
	return &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		dialer := net.Dialer{Timeout: 15 * time.Second}
		for _, ip := range ips {
			if !allowPrivate && blockedIP(ip) {
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, fmt.Errorf("no permitted IP address for %s", host)
	}}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("OpenAPI redirects are disabled") }}
}
