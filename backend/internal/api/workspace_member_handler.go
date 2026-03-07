package api

import (
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// WorkspaceMemberHandler handles workspace membership endpoints.
type WorkspaceMemberHandler struct {
	memberRepo *repository.WorkspaceMemberRepo
	userRepo   *repository.UserRepo
}

// NewWorkspaceMemberHandler creates a new WorkspaceMemberHandler.
func NewWorkspaceMemberHandler(memberRepo *repository.WorkspaceMemberRepo, userRepo *repository.UserRepo) *WorkspaceMemberHandler {
	return &WorkspaceMemberHandler{
		memberRepo: memberRepo,
		userRepo:   userRepo,
	}
}

// ListMembers returns all members of a workspace.
func (h *WorkspaceMemberHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")

	members, err := h.memberRepo.ListByWorkspace(workspaceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	if members == nil {
		respondJSON(w, http.StatusOK, []interface{}{})
		return
	}

	// Enrich with user info
	type memberResponse struct {
		WorkspaceID string `json:"workspace_id"`
		UserID      string `json:"user_id"`
		Role        string `json:"role"`
		JoinedAt    string `json:"joined_at"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}

	var result []memberResponse
	for _, m := range members {
		resp := memberResponse{
			WorkspaceID: m.WorkspaceID,
			UserID:      m.UserID,
			Role:        m.Role,
			JoinedAt:    m.JoinedAt.Format("2006-01-02T15:04:05Z"),
		}
		// Enrich with user display info
		user, err := h.userRepo.GetByID(m.UserID)
		if err == nil && user != nil {
			resp.Username = user.Username
			resp.DisplayName = user.DisplayName
		}
		result = append(result, resp)
	}

	respondJSON(w, http.StatusOK, result)
}

type addMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// AddMember adds a user to a workspace.
func (h *WorkspaceMemberHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")

	var req addMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}

	// Verify user exists
	user, err := h.userRepo.GetByID(req.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to find user")
		return
	}
	if user == nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	if err := h.memberRepo.Add(workspaceID, req.UserID, req.Role); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add member")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"workspace_id": workspaceID,
		"user_id":      req.UserID,
		"role":         req.Role,
	})
}

type updateMemberRoleRequest struct {
	Role string `json:"role"`
}

// UpdateMemberRole changes a member's role in a workspace.
func (h *WorkspaceMemberHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")

	var req updateMemberRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role == "" {
		respondError(w, http.StatusBadRequest, "role is required")
		return
	}

	if err := h.memberRepo.UpdateRole(workspaceID, userID, req.Role); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update member role")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"workspace_id": workspaceID,
		"user_id":      userID,
		"role":         req.Role,
	})
}

// RemoveMember removes a user from a workspace.
func (h *WorkspaceMemberHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")

	if err := h.memberRepo.Remove(workspaceID, userID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
