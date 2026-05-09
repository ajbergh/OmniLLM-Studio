// Package mcpclient implements the Model Context Protocol (MCP).
// It handles server lifecycle management, protocol handshakes, tool discovery,
// and maps remote MCP tools to the local OmniLLM-Studio tool registry.
package mcpclient

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// Manager owns MCP server runtime state and dynamic tool registration.
type Manager struct {
	repo     *repository.MCPServerRepo
	permRepo *repository.ToolPermissionRepo
	registry *tools.Registry

	mu      sync.RWMutex
	servers map[string]*serverState
}

type serverState struct {
	status          models.MCPServerStatus
	lastError       string
	client          MCPClient
	registeredNames []string
	tools           []models.MCPTool
}

// NewManager creates an MCP runtime manager.
func NewManager(repo *repository.MCPServerRepo, permRepo *repository.ToolPermissionRepo, registry *tools.Registry) *Manager {
	return &Manager{
		repo:     repo,
		permRepo: permRepo,
		registry: registry,
		servers:  make(map[string]*serverState),
	}
}

// StartEnabled starts every enabled MCP server from persisted config.
func (m *Manager) StartEnabled(ctx context.Context) {
	servers, err := m.repo.ListRuntime()
	if err != nil {
		log.Printf("[mcp] list enabled servers: %v", err)
		return
	}
	for _, server := range servers {
		if !server.Enabled {
			continue
		}
		startCtx, cancel := context.WithTimeout(ctx, time.Duration(defaultRequestTimeout)*time.Second)
		if err := m.Start(startCtx, server.ID); err != nil {
			log.Printf("[mcp] start %s: %v", server.Name, err)
		}
		cancel()
	}
}

// StopAll stops all running MCP servers.
func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	ids := make([]string, 0, len(m.servers))
	for id := range m.servers {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	var firstErr error
	for _, id := range ids {
		if err := m.Stop(ctx, id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ListServers returns all configured servers enriched with runtime status.
func (m *Manager) ListServers() ([]models.MCPServerWithStatus, error) {
	servers, err := m.repo.List()
	if err != nil {
		return nil, err
	}
	out := make([]models.MCPServerWithStatus, 0, len(servers))
	for _, server := range servers {
		out = append(out, m.withStatus(server))
	}
	return out, nil
}

// GetServer returns one configured server enriched with runtime status.
func (m *Manager) GetServer(id string) (*models.MCPServerWithStatus, error) {
	server, err := m.repo.GetByID(id)
	if err != nil || server == nil {
		return nil, err
	}
	out := m.withStatus(*server)
	return &out, nil
}

// Start launches a configured server, discovers tools, and registers adapters.
func (m *Manager) Start(ctx context.Context, id string) error {
	server, err := m.repo.GetRuntimeByID(id)
	if err != nil {
		return err
	}
	if server == nil {
		return sql.ErrNoRows
	}
	if err := validateRuntimeServer(*server); err != nil {
		m.setError(id, err)
		return err
	}

	if err := m.Stop(ctx, id); err != nil {
		log.Printf("[mcp] stop before restart %s: %v", server.Name, err)
	}

	m.setStatus(id, models.MCPServerStatusConnecting, "")

	client := newMCPClient(*server)
	if err := client.Start(ctx); err != nil {
		m.setError(id, err)
		m.audit(models.MCPAuditEvent{ServerID: id, EventType: "error", ErrorMsg: stringPtr(err.Error())})
		return err
	}

	discovered, err := client.ListTools(ctx)
	if err != nil {
		_ = client.Stop(context.Background())
		m.setError(id, err)
		m.audit(models.MCPAuditEvent{ServerID: id, EventType: "error", ErrorMsg: stringPtr(err.Error())})
		return err
	}

	registered, mcpTools, err := m.registerTools(*server, client, discovered)
	if err != nil {
		for _, name := range registered {
			m.registry.Remove(name)
		}
		_ = client.Stop(context.Background())
		m.setError(id, err)
		m.audit(models.MCPAuditEvent{ServerID: id, EventType: "error", ErrorMsg: stringPtr(err.Error())})
		return err
	}

	m.mu.Lock()
	m.servers[id] = &serverState{
		status:          models.MCPServerStatusConnected,
		client:          client,
		registeredNames: registered,
		tools:           mcpTools,
	}
	m.mu.Unlock()

	m.audit(models.MCPAuditEvent{ServerID: id, EventType: "server_start"})
	return nil
}

// Stop terminates a configured server runtime and unregisters its tools.
func (m *Manager) Stop(ctx context.Context, id string) error {
	m.mu.Lock()
	state, ok := m.servers[id]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.servers, id)
	m.mu.Unlock()

	for _, name := range state.registeredNames {
		m.registry.Remove(name)
	}

	var err error
	if state.client != nil {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = state.client.Stop(stopCtx)
		cancel()
	}

	m.audit(models.MCPAuditEvent{ServerID: id, EventType: "server_stop"})
	if err != nil {
		return err
	}
	return nil
}

// Restart stops and starts a server.
func (m *Manager) Restart(ctx context.Context, id string) error {
	if err := m.Stop(ctx, id); err != nil {
		return err
	}
	return m.Start(ctx, id)
}

// RefreshTools re-discovers tools by restarting the server.
func (m *Manager) RefreshTools(ctx context.Context, id string) ([]models.MCPTool, error) {
	if err := m.Restart(ctx, id); err != nil {
		return nil, err
	}
	return m.ListTools(id)
}

// Test starts a temporary client and returns discovered tools without registering them.
func (m *Manager) Test(ctx context.Context, id string) ([]models.MCPTool, error) {
	server, err := m.repo.GetRuntimeByID(id)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, sql.ErrNoRows
	}
	if err := validateRuntimeServer(*server); err != nil {
		return nil, err
	}

	client := newMCPClient(*server)
	if err := client.Start(ctx); err != nil {
		return nil, err
	}
	defer client.Stop(context.Background())

	discovered, err := client.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.MCPTool, 0, len(discovered))
	used := make(map[string]bool, len(discovered))
	for _, toolDef := range discovered {
		internalName := uniqueToolName(BuildToolName(server.Name, toolDef.Name), used, nil)
		used[internalName] = true
		out = append(out, m.toMCPTool(server.ID, internalName, toolDef, "ask"))
	}
	return out, nil
}

// ListTools returns the tools discovered for a running server.
func (m *Manager) ListTools(id string) ([]models.MCPTool, error) {
	server, err := m.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, sql.ErrNoRows
	}

	m.mu.RLock()
	state := m.servers[id]
	m.mu.RUnlock()
	if state == nil {
		return []models.MCPTool{}, nil
	}

	toolsOut := make([]models.MCPTool, len(state.tools))
	for i, toolDef := range state.tools {
		toolDef.Policy = m.policyFor(toolDef.InternalName, "ask")
		toolsOut[i] = toolDef
	}
	return toolsOut, nil
}

// SetToolPolicy updates a discovered MCP tool's execution policy.
func (m *Manager) SetToolPolicy(serverID, internalName, policy string) error {
	if policy != "allow" && policy != "deny" && policy != "ask" {
		return fmt.Errorf("policy must be allow, deny, or ask")
	}
	if !m.toolBelongsToServer(serverID, internalName) {
		return sql.ErrNoRows
	}
	return m.permRepo.Upsert(internalName, policy)
}

func (m *Manager) withStatus(server models.MCPServer) models.MCPServerWithStatus {
	out := models.MCPServerWithStatus{MCPServer: server}
	m.mu.RLock()
	state := m.servers[server.ID]
	m.mu.RUnlock()

	if state == nil {
		if server.Enabled {
			out.Status = models.MCPServerStatusDisconnected
		} else {
			out.Status = models.MCPServerStatusDisabled
		}
		return out
	}

	out.Status = state.status
	out.LastError = state.lastError
	out.Tools = make([]models.MCPTool, len(state.tools))
	for i, toolDef := range state.tools {
		toolDef.Policy = m.policyFor(toolDef.InternalName, "ask")
		out.Tools[i] = toolDef
	}
	return out
}

func (m *Manager) registerTools(server models.MCPServer, client MCPClient, discovered []Tool) ([]string, []models.MCPTool, error) {
	registered := make([]string, 0, len(discovered))
	mcpTools := make([]models.MCPTool, 0, len(discovered))
	used := make(map[string]bool, len(discovered))

	for _, toolDef := range discovered {
		baseName := BuildToolName(server.Name, toolDef.Name)
		internalName := uniqueToolName(baseName, used, m.registry.Get)
		used[internalName] = true

		if err := m.seedDefaultPolicy(internalName); err != nil {
			return registered, mcpTools, err
		}

		adapter := NewToolAdapter(server.ID, server.Name, internalName, toolDef, client, m.audit)
		if err := m.registry.Register(adapter); err != nil {
			return registered, mcpTools, err
		}
		registered = append(registered, internalName)
		mcpTools = append(mcpTools, m.toMCPTool(server.ID, internalName, toolDef, m.policyFor(internalName, "ask")))
	}

	return registered, mcpTools, nil
}

func (m *Manager) seedDefaultPolicy(toolName string) error {
	existing, err := m.permRepo.Get(toolName)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	return m.permRepo.Upsert(toolName, "ask")
}

func (m *Manager) toMCPTool(serverID, internalName string, toolDef Tool, policy string) models.MCPTool {
	return models.MCPTool{
		ServerID:     serverID,
		InternalName: internalName,
		Name:         toolDef.Name,
		Title:        toolDef.Title,
		Description:  toolDef.Description,
		InputSchema:  toolDef.InputSchema,
		Policy:       policy,
		Enabled:      true,
	}
}

func (m *Manager) policyFor(toolName, fallback string) string {
	permission, err := m.permRepo.Get(toolName)
	if err != nil || permission == nil || permission.Policy == "" {
		return fallback
	}
	return permission.Policy
}

func (m *Manager) toolBelongsToServer(serverID, internalName string) bool {
	m.mu.RLock()
	state := m.servers[serverID]
	m.mu.RUnlock()
	if state == nil {
		return false
	}
	for _, toolDef := range state.tools {
		if toolDef.InternalName == internalName {
			return true
		}
	}
	return false
}

func (m *Manager) setStatus(id string, status models.MCPServerStatus, lastError string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.servers[id]
	if state == nil {
		state = &serverState{}
		m.servers[id] = state
	}
	state.status = status
	state.lastError = lastError
}

func (m *Manager) setError(id string, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	m.setStatus(id, models.MCPServerStatusError, message)
}

func (m *Manager) audit(event models.MCPAuditEvent) {
	if err := m.repo.InsertAudit(event); err != nil {
		log.Printf("[mcp] audit insert failed: %v", err)
	}
}

func validateRuntimeServer(server models.MCPServer) error {
	switch server.Transport {
	case "", "stdio":
		if server.Command == nil || *server.Command == "" {
			return fmt.Errorf("command is required for stdio MCP server")
		}
	case "http":
		if server.URL == nil || *server.URL == "" {
			return fmt.Errorf("url is required for http MCP server")
		}
	default:
		return fmt.Errorf("unsupported MCP transport %q", server.Transport)
	}
	return nil
}

// newMCPClient selects the correct MCPClient implementation based on transport.
func newMCPClient(server models.MCPServer) MCPClient {
	if server.Transport == "http" {
		return NewHTTPClient(server)
	}
	return NewClient(server)
}

type registryLookup func(name string) (tools.Tool, bool)

func uniqueToolName(base string, used map[string]bool, lookup registryLookup) string {
	candidate := base
	for i := 1; ; i++ {
		inUse := used[candidate]
		if !inUse && lookup != nil {
			_, inUse = lookup(candidate)
		}
		if !inUse {
			return candidate
		}

		suffix := "_" + strconv.Itoa(i)
		maxBase := maxInternalToolNameLen - len(suffix)
		if maxBase < 1 {
			maxBase = 1
		}
		if len(base) > maxBase {
			candidate = base[:maxBase] + suffix
		} else {
			candidate = base + suffix
		}
	}
}

func stringPtr(value string) *string {
	return &value
}
