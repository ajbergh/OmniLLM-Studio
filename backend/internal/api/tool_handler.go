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

// ToolHandler exposes tool management and pending-approval endpoints.
type ToolHandler struct {
	registry *tools.Registry
	executor *tools.Executor
	permRepo *repository.ToolPermissionRepo
}

func NewToolHandler(registry *tools.Registry, executor *tools.Executor, permRepo *repository.ToolPermissionRepo) *ToolHandler {
	return &ToolHandler{registry: registry, executor: executor, permRepo: permRepo}
}

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
	for i, definition := range defs {
		policy := policyMap[definition.Name]
		if policy == "" {
			if definition.SideEffecting || definition.Risk == tools.RiskHigh || definition.Risk == tools.RiskCritical {
				policy = "ask"
			} else {
				policy = "allow"
			}
		}
		out[i] = toolInfo{ToolDefinition: definition, Policy: policy}
	}
	respondJSON(w, http.StatusOK, out)
}

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

// ExecuteTool supports ordinary invocation plus approval and runtime metrics
// actions through the existing authenticated route, preserving compatibility
// with deployed routers.
func (h *ToolHandler) ExecuteTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action     string          `json:"action,omitempty"`
		Name       string          `json:"name,omitempty"`
		Arguments  json.RawMessage `json:"arguments,omitempty"`
		ApprovalID string          `json:"approval_id,omitempty"`
		Approved   bool            `json:"approved,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	userID := auth.ScopeUserIDFromContext(r.Context())
	switch req.Action {
	case "list_approvals":
		respondJSON(w, http.StatusOK, h.executor.ApprovalBroker().List(tools.InvocationScope{UserID: userID}))
		return
	case "resolve_approval":
		result, err := h.resolveAndExecuteApproval(r, userID, req.ApprovalID, req.Approved, req.Arguments)
		if err != nil {
			respondApprovalError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"approval_id": req.ApprovalID, "approved": req.Approved, "result": result})
		return
	case "metrics":
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"scope": "process",
			"tools": tools.ToolMetricsSnapshot(),
		})
		return
	case "":
	default:
		respondError(w, http.StatusBadRequest, "unsupported action")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.Arguments) == 0 {
		req.Arguments = json.RawMessage(`{}`)
	}
	ctx := tools.ContextWithInvocationScope(r.Context(), tools.InvocationScope{UserID: userID})
	result := h.executor.Execute(ctx, tools.ToolCall{ID: "manual-" + uuid.NewString(), Name: req.Name, Arguments: req.Arguments})
	if result.IsError {
		respondJSON(w, http.StatusUnprocessableEntity, result)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *ToolHandler) ListApprovals(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.executor.ApprovalBroker().List(tools.InvocationScope{UserID: auth.ScopeUserIDFromContext(r.Context())}))
}

func (h *ToolHandler) ResolveApproval(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Approved  bool            `json:"approved"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	approvalID := chi.URLParam(r, "approvalId")
	result, err := h.resolveAndExecuteApproval(r, auth.ScopeUserIDFromContext(r.Context()), approvalID, req.Approved, req.Arguments)
	if err != nil {
		respondApprovalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"approval_id": approvalID, "approved": req.Approved, "result": result})
}

func (h *ToolHandler) resolveAndExecuteApproval(r *http.Request, userID, approvalID string, approved bool, editedArguments json.RawMessage) (*tools.ToolResult, error) {
	if approvalID == "" {
		return nil, errors.New("approval_id is required")
	}
	pending, ok := h.executor.ApprovalBroker().Get(approvalID)
	if !ok {
		return nil, tools.ErrApprovalNotFound
	}
	if pending.Request.Scope.UserID != "" && pending.Request.Scope.UserID != userID {
		return nil, errors.New("approval does not belong to current user")
	}
	arguments := pending.Request.Arguments
	if len(editedArguments) > 0 {
		if !json.Valid(editedArguments) {
			return nil, errors.New("arguments must be valid JSON")
		}
		arguments = editedArguments
	}
	if err := h.executor.ApprovalBroker().Resolve(approvalID, approved, arguments); err != nil {
		return nil, err
	}
	if !approved || pending.Request.ContinuationMode == "inline" {
		// Inline Chat Studio approvals resume inside the original executor call.
		// The waiting SSE request will emit the eventual tool result and continue
		// the same model turn, so this endpoint must not execute the tool twice.
		return nil, nil
	}
	ctx := tools.ContextWithInvocationScope(r.Context(), pending.Request.Scope)
	result := h.executor.ExecuteApproved(ctx, tools.ToolCall{
		ID: pending.Request.ToolCallID, Name: pending.Request.ToolName, Arguments: arguments,
	})
	return result, nil
}

func respondApprovalError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tools.ErrApprovalNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, tools.ErrApprovalResolved):
		respondError(w, http.StatusConflict, err.Error())
	default:
		respondError(w, http.StatusBadRequest, err.Error())
	}
}
