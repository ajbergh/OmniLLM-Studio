package api

import (
	"encoding/json"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
	"github.com/go-chi/chi/v5"
)

// ToolHandler exposes tool management endpoints.
type ToolHandler struct {
	registry *tools.Registry
	executor *tools.Executor
	permRepo *repository.ToolPermissionRepo
}

// NewToolHandler creates a ToolHandler.
func NewToolHandler(
	registry *tools.Registry,
	executor *tools.Executor,
	permRepo *repository.ToolPermissionRepo,
) *ToolHandler {
	return &ToolHandler{
		registry: registry,
		executor: executor,
		permRepo: permRepo,
	}
}

// ListTools returns all registered tool definitions along with their
// current permission policy.
func (h *ToolHandler) ListTools(w http.ResponseWriter, r *http.Request) {
	defs := h.registry.List()
	perms, err := h.permRepo.List()
	if err != nil {
		respondInternalError(w, err)
		return
	}

	// Build permission lookup
	policyMap := make(map[string]string, len(perms))
	for _, p := range perms {
		policyMap[p.ToolName] = p.Policy
	}

	type toolInfo struct {
		tools.ToolDefinition
		Policy string `json:"policy"`
	}

	out := make([]toolInfo, len(defs))
	for i, d := range defs {
		policy := policyMap[d.Name]
		if policy == "" {
			policy = "allow"
		}
		out[i] = toolInfo{ToolDefinition: d, Policy: policy}
	}

	respondJSON(w, http.StatusOK, out)
}

// UpdatePermission sets the policy for a specific tool.
func (h *ToolHandler) UpdatePermission(w http.ResponseWriter, r *http.Request) {
	toolName := chi.URLParam(r, "toolName")
	if _, ok := h.registry.Get(toolName); !ok {
		respondError(w, http.StatusNotFound, "unknown tool: "+toolName)
		return
	}

	var req struct {
		Policy string `json:"policy"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Policy {
	case "allow", "deny", "ask":
		// valid
	default:
		respondError(w, http.StatusBadRequest, "policy must be allow, deny, or ask")
		return
	}

	if err := h.permRepo.Upsert(toolName, req.Policy); err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"tool_name": toolName,
		"policy":    req.Policy,
	})
}

// ExecuteTool manually invokes a tool by name with the given arguments.
func (h *ToolHandler) ExecuteTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	call := tools.ToolCall{
		ID:        "manual-" + req.Name,
		Name:      req.Name,
		Arguments: req.Arguments,
	}

	result := h.executor.Execute(r.Context(), call)
	if result.IsError {
		respondJSON(w, http.StatusUnprocessableEntity, result)
		return
	}
	respondJSON(w, http.StatusOK, result)
}
