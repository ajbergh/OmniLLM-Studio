package api

import (
	"net/http"

	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// BranchHandler provides API endpoints for conversation branching.
type BranchHandler struct {
	branchRepo *repository.BranchRepo
	msgRepo    *repository.MessageRepo
	convoRepo  *repository.ConversationRepo
}

// NewBranchHandler creates a new BranchHandler.
func NewBranchHandler(branchRepo *repository.BranchRepo, msgRepo *repository.MessageRepo, convoRepo *repository.ConversationRepo) *BranchHandler {
	return &BranchHandler{
		branchRepo: branchRepo,
		msgRepo:    msgRepo,
		convoRepo:  convoRepo,
	}
}

// createBranchRequest is the JSON body for creating a branch.
type createBranchRequest struct {
	Name          string `json:"name"`
	ForkMessageID string `json:"fork_message_id"`
}

// ListBranches returns all branches for a conversation.
func (h *BranchHandler) ListBranches(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	conversationID := chi.URLParam(r, "conversationId")

	branches, err := h.branchRepo.List(conversationID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, branches)
}

// CreateBranch creates a new branch from a specific message.
func (h *BranchHandler) CreateBranch(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	conversationID := chi.URLParam(r, "conversationId")

	var req createBranchRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ForkMessageID == "" {
		respondError(w, http.StatusBadRequest, "fork_message_id is required")
		return
	}

	// Verify the fork message exists and belongs to this conversation
	msg, err := h.msgRepo.GetByID(req.ForkMessageID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if msg == nil || msg.ConversationID != conversationID {
		respondError(w, http.StatusNotFound, "fork message not found in conversation")
		return
	}

	name := req.Name
	if name == "" {
		name = "Branch"
	}

	branch, err := h.branchRepo.Create(conversationID, name, "main", req.ForkMessageID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, branch)
}

// DeleteBranch removes a branch and its messages.
func (h *BranchHandler) DeleteBranch(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	branchID := chi.URLParam(r, "branchId")

	branch, err := h.branchRepo.GetByID(branchID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if branch == nil {
		respondError(w, http.StatusNotFound, "branch not found")
		return
	}

	if err := h.branchRepo.Delete(branchID); err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// RenameBranch updates a branch's name.
func (h *BranchHandler) RenameBranch(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	branchID := chi.URLParam(r, "branchId")

	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := h.branchRepo.Rename(branchID, req.Name); err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ListBranchMessages returns messages for a specific branch.
func (h *BranchHandler) ListBranchMessages(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	conversationID := chi.URLParam(r, "conversationId")
	branchID := r.URL.Query().Get("branch")

	messages, err := h.msgRepo.ListByBranch(conversationID, branchID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, messages)
}
