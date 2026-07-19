package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
	return &ToolHandler{registry: registry, executor: executor, permRepo: permRepo}
}

// ListTools returns all registered tool definitions along with their current
// permission policy.
func (h *ToolHandler) ListTools(w http.ResponseWriter, r *http.Request) {
	defs := h.registry.List()
	perms, err := h.permRepo.List()
	if err != nil {
		respondInternalError(w, err)
		return
	}

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
			if d.SideEffecting || d.Risk == tools.RiskHigh || d.Risk == tools.RiskCritical {
				policy = "ask"
			} else {
				policy = "allow"
			}
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
	default:
		respondError(w, http.StatusBadRequest, "policy must be allow, deny, or ask")
		return
	}
	if err := h.permRepo.Upsert(toolName, req.Policy); err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"tool_name": toolName, "policy": req.Policy})
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
	if len(req.Arguments) == 0 {
		req.Arguments = json.RawMessage(`{}`)
	}

	call := tools.ToolCall{ID: "manual-" + uuid.NewString(), Name: req.Name, Arguments: req.Arguments}
	ctx := tools.ContextWithInvocationScope(r.Context(), tools.InvocationScope{
		UserID: auth.UserIDFromContext(r.Context()),
	})
	result := h.executor.Execute(ctx, call)
	if result.IsError {
		respondJSON(w, http.StatusUnprocessableEntity, result)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

// ListApprovals lists pending tool approvals owned by the current user.
func (h *ToolHandler) ListApprovals(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	respondJSON(w, http.StatusOK, h.executor.ApprovalBroker().List(tools.InvocationScope{UserID: userID}))
}

// ResolveApproval approves or rejects a pending invocation. Users may edit the
// arguments before approval; the executor validates the replacement contract.
func (h *ToolHandler) ResolveApproval(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "approvalId")
	pending, ok := h.executor.ApprovalBroker().Get(approvalID)
	if !ok {
		respondError(w, http.StatusNotFound, "approval not found")
		return
	}
	userID := auth.UserIDFromContext(r.Context())
	if pending.Request.Scope.UserID != "" && pending.Request.Scope.UserID != userID {
		respondError(w, http.StatusForbidden, "approval does not belong to current user")
		return
	}

	var req struct {
		Approved  bool            `json:"approved"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Arguments) > 0 && !json.Valid(req.Arguments) {
		respondError(w, http.StatusBadRequest, "arguments must be valid JSON")
		return
	}
	if err := h.executor.ApprovalBroker().Resolve(approvalID, req.Approved, req.Arguments); err != nil {
		switch {
		case errors.Is(err, tools.ErrApprovalNotFound):
			respondError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, tools.ErrApprovalResolved):
			respondError(w, http.StatusConflict, err.Error())
		default:
			respondError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"approval_id": approvalID,
		"approved":    req.Approved,
	})
}
