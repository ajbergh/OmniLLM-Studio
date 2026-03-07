package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	maxUploadSize = 10 << 20 // 10 MB
)

// allowedMIMEPrefixes defines MIME types accepted for upload.
var allowedMIMEPrefixes = []string{
	"text/",
	"image/",
	"audio/",
	"video/",
	"application/pdf",
	"application/json",
	"application/xml",
	"application/zip",
	"application/gzip",
	"application/x-tar",
	"application/octet-stream",
	"application/vnd.",
	"application/x-yaml",
	"application/yaml",
}

func isAllowedMIME(mimeType string) bool {
	lower := strings.ToLower(mimeType)
	for _, prefix := range allowedMIMEPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// AttachmentHandler handles file attachment API endpoints.
type AttachmentHandler struct {
	repo       *repository.AttachmentRepo
	convoRepo  *repository.ConversationRepo
	storageDir string
}

// NewAttachmentHandler creates a new AttachmentHandler.
func NewAttachmentHandler(repo *repository.AttachmentRepo, convoRepo *repository.ConversationRepo, storageDir string) *AttachmentHandler {
	// Ensure storage directory exists.
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		log.Printf("[attachments] warning: could not create storage dir %s: %v", storageDir, err)
	}
	return &AttachmentHandler{repo: repo, convoRepo: convoRepo, storageDir: storageDir}
}

// Upload handles multipart file upload.
// POST /v1/conversations/{conversationId}/attachments
func (h *AttachmentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	conversationID := chi.URLParam(r, "conversationId")

	// Limit request body size.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("file too large (max %d MB)", maxUploadSize>>20))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing 'file' field in multipart form")
		return
	}
	defer file.Close()

	// Determine MIME type.
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		ext := filepath.Ext(header.Filename)
		if guessed := mime.TypeByExtension(ext); guessed != "" {
			mimeType = guessed
		} else {
			mimeType = "application/octet-stream"
		}
	}

	// Validate MIME type against allowlist.
	if !isAllowedMIME(mimeType) {
		respondError(w, http.StatusBadRequest, "file type not allowed")
		return
	}

	// Determine attachment type.
	attachType := "file"
	if strings.HasPrefix(mimeType, "image/") {
		attachType = "image"
	}

	// Generate a UUID-based storage filename (preserve extension for serving).
	ext := filepath.Ext(header.Filename)
	storageFilename := uuid.New().String() + ext
	storagePath := filepath.Join(h.storageDir, storageFilename)

	out, err := os.Create(storagePath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	defer out.Close()

	written, err := io.Copy(out, file)
	if err != nil {
		os.Remove(storagePath)
		respondError(w, http.StatusInternalServerError, "failed to write file")
		return
	}

	metaObj := map[string]interface{}{"original_name": header.Filename}
	metaBytes, _ := json.Marshal(metaObj)

	attachment := &models.Attachment{
		ConversationID: conversationID,
		Type:           attachType,
		MimeType:       mimeType,
		StoragePath:    storageFilename, // store relative name, not full path
		Bytes:          written,
		MetadataJSON:   string(metaBytes),
	}

	if err := h.repo.Create(attachment); err != nil {
		os.Remove(storagePath)
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, attachment)
}

// List returns all attachments for a conversation.
// GET /v1/conversations/{conversationId}/attachments
func (h *AttachmentHandler) List(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	conversationID := chi.URLParam(r, "conversationId")

	attachments, err := h.repo.ListByConversation(conversationID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	if attachments == nil {
		attachments = []models.Attachment{}
	}
	respondJSON(w, http.StatusOK, attachments)
}

// Download serves the raw file.
// GET /v1/attachments/{attachmentId}/download
func (h *AttachmentHandler) Download(w http.ResponseWriter, r *http.Request) {
	attachmentID := chi.URLParam(r, "attachmentId")

	attachment, err := h.repo.GetByID(attachmentID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if attachment == nil {
		respondError(w, http.StatusNotFound, "attachment not found")
		return
	}

	// Verify user owns the parent conversation
	if !verifyConversationAccessByID(w, r, h.convoRepo, attachment.ConversationID) {
		return
	}

	filePath, pathErr := SafeJoin(h.storageDir, attachment.StoragePath)
	if pathErr != nil {
		respondError(w, http.StatusBadRequest, "invalid attachment path")
		return
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		respondError(w, http.StatusNotFound, "file not found on disk")
		return
	}

	w.Header().Set("Content-Type", attachment.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(attachment.StoragePath)))
	http.ServeFile(w, r, filePath)
}

// Delete removes an attachment record and its file.
// DELETE /v1/attachments/{attachmentId}
func (h *AttachmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	attachmentID := chi.URLParam(r, "attachmentId")

	attachment, err := h.repo.GetByID(attachmentID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if attachment == nil {
		respondError(w, http.StatusNotFound, "attachment not found")
		return
	}

	// Verify user owns the parent conversation
	if !verifyConversationAccessByID(w, r, h.convoRepo, attachment.ConversationID) {
		return
	}

	// Delete DB record.
	if err := h.repo.Delete(attachmentID); err != nil {
		respondInternalError(w, err)
		return
	}

	// Remove file from disk (best-effort) — use safe path joining.
	filePath, pathErr := SafeJoin(h.storageDir, attachment.StoragePath)
	if pathErr == nil {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			log.Printf("[attachments] warning: failed to remove file %s: %v", filePath, err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
