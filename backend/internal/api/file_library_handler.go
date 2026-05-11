package api

import (
	"net/http"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/filelibrary"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// FileLibraryHandler exposes file-library APIs.
type FileLibraryHandler struct {
	svc       *filelibrary.LibraryService
	convoRepo *repository.ConversationRepo
}

// NewFileLibraryHandler creates a new FileLibraryHandler.
func NewFileLibraryHandler(svc *filelibrary.LibraryService, convoRepo *repository.ConversationRepo) *FileLibraryHandler {
	return &FileLibraryHandler{svc: svc, convoRepo: convoRepo}
}

// ListFiles handles GET /v1/file-library/files
func (h *FileLibraryHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	files, err := h.svc.ListFiles(r.Context(), userID, scope, query)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if files == nil {
		files = []models.LibraryFile{}
	}
	respondJSON(w, http.StatusOK, files)
}

// IngestFile handles POST /v1/file-library/files/ingest
func (h *FileLibraryHandler) IngestFile(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	var req filelibrary.IngestFileRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.OwnerUserID = userID
	if req.AttachmentID == "" {
		respondError(w, http.StatusBadRequest, "attachment_id is required")
		return
	}
	file, err := h.svc.IngestFile(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, file)
}

// Search handles POST /v1/file-library/search
func (h *FileLibraryHandler) Search(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	var req filelibrary.SearchRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.OwnerUserID = userID
	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// Fetch handles GET /v1/file-library/files/{fileId}
func (h *FileLibraryHandler) Fetch(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	fileID := chi.URLParam(r, "fileId")
	resp, err := h.svc.Fetch(r.Context(), filelibrary.FetchRequest{
		OwnerUserID:     userID,
		LibraryFileID:   fileID,
		IncludeFullText: strings.EqualFold(r.URL.Query().Get("include_full_text"), "true"),
	})
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// Update handles PATCH /v1/file-library/files/{fileId}
func (h *FileLibraryHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	fileID := chi.URLParam(r, "fileId")
	var body struct {
		DisplayName *string                `json:"display_name,omitempty"`
		Scope       *string                `json:"scope,omitempty"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := h.svc.UpdateFile(r.Context(), filelibrary.UpdateFileRequest{
		OwnerUserID:   userID,
		LibraryFileID: fileID,
		DisplayName:   body.DisplayName,
		Scope:         body.Scope,
		Metadata:      body.Metadata,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /v1/file-library/files/{fileId}
func (h *FileLibraryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	fileID := chi.URLParam(r, "fileId")
	hardDelete := strings.EqualFold(r.URL.Query().Get("hard"), "true")
	if err := h.svc.DeleteFile(r.Context(), filelibrary.DeleteFileRequest{
		OwnerUserID:   userID,
		LibraryFileID: fileID,
		HardDelete:    hardDelete,
	}); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "hard": hardDelete})
}

// Reindex handles POST /v1/file-library/files/{fileId}/reindex
func (h *FileLibraryHandler) Reindex(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	fileID := chi.URLParam(r, "fileId")
	resp, err := h.svc.ReindexFile(r.Context(), filelibrary.ReindexFileRequest{
		OwnerUserID:   userID,
		LibraryFileID: fileID,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// Summarize handles POST /v1/file-library/summarize
func (h *FileLibraryHandler) Summarize(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	var req filelibrary.SummarizeRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.OwnerUserID = userID
	if len(req.LibraryFileIDs) == 0 {
		respondError(w, http.StatusBadRequest, "library_file_ids is required")
		return
	}
	resp, err := h.svc.Summarize(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// Compare handles POST /v1/file-library/compare
func (h *FileLibraryHandler) Compare(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	var req filelibrary.CompareRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.OwnerUserID = userID
	if len(req.LibraryFileIDs) < 2 {
		respondError(w, http.StatusBadRequest, "at least two library_file_ids are required")
		return
	}
	resp, err := h.svc.Compare(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// ConversationSearch handles POST /v1/conversations/{conversationId}/file-library/search
func (h *FileLibraryHandler) ConversationSearch(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	userID := auth.UserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "conversationId")
	var req filelibrary.SearchRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.OwnerUserID = userID
	req.ConversationID = conversationID
	if strings.TrimSpace(req.Scope) == "" {
		req.Scope = "conversation"
	}
	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// ConversationListFiles handles GET /v1/conversations/{conversationId}/file-library/files
func (h *FileLibraryHandler) ConversationListFiles(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	userID := auth.UserIDFromContext(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	files, err := h.svc.ListFiles(r.Context(), userID, "conversation", query)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if files == nil {
		files = []models.LibraryFile{}
	}
	respondJSON(w, http.StatusOK, files)
}

// ConversationIngestAttachment handles POST /v1/conversations/{conversationId}/file-library/ingest-attachments
func (h *FileLibraryHandler) ConversationIngestAttachment(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	userID := auth.UserIDFromContext(r.Context())
	conversationID := chi.URLParam(r, "conversationId")
	var req struct {
		AttachmentIDs []string `json:"attachment_ids"`
		Scope         string   `json:"scope,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.AttachmentIDs) == 0 {
		respondError(w, http.StatusBadRequest, "attachment_ids is required")
		return
	}
	scope := req.Scope
	if strings.TrimSpace(scope) == "" {
		scope = "conversation"
	}
	out := make([]interface{}, 0, len(req.AttachmentIDs))
	for _, attachmentID := range req.AttachmentIDs {
		file, err := h.svc.IngestFile(r.Context(), filelibrary.IngestFileRequest{
			OwnerUserID:    userID,
			AttachmentID:   attachmentID,
			Scope:          scope,
			ConversationID: conversationID,
		})
		if err != nil {
			out = append(out, map[string]interface{}{"attachment_id": attachmentID, "error": err.Error()})
			continue
		}
		out = append(out, file)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"results": out})
}
