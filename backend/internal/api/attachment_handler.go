package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/filelibrary"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxUploadSize = 50 << 20 // 50 MB

var allowedExactMIMETypes = map[string]bool{
	"application/pdf": true,
	"application/json": true,
	"application/xml": true,
	"application/yaml": true,
	"application/x-yaml": true,
	"application/zip": true,
	"application/gzip": true,
	"application/x-gzip": true,
	"application/x-tar": true,
	"application/rtf": true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	"application/vnd.ms-excel": true,
	"application/vnd.ms-powerpoint": true,
	"application/vnd.oasis.opendocument.text": true,
	"application/vnd.oasis.opendocument.spreadsheet": true,
	"application/vnd.oasis.opendocument.presentation": true,
}

func isAllowedMIME(mimeType string) bool {
	mimeType = normalizeMIME(mimeType)
	return strings.HasPrefix(mimeType, "text/") ||
		strings.HasPrefix(mimeType, "image/") ||
		strings.HasPrefix(mimeType, "audio/") ||
		strings.HasPrefix(mimeType, "video/") ||
		allowedExactMIMETypes[mimeType]
}

func normalizeMIME(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
}

// detectUploadedMIME validates the declared type against the file bytes. ZIP-
// based office documents are allowed to use the more specific extension-derived
// MIME type because net/http can only identify the outer ZIP container.
func detectUploadedMIME(file multipart.File, header *multipart.FileHeader) (string, error) {
	head := make([]byte, 512)
	n, readErr := io.ReadFull(file, head)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("read file header: %w", readErr)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind upload: %w", err)
	}

	sniffed := normalizeMIME(http.DetectContentType(head[:n]))
	declared := normalizeMIME(header.Header.Get("Content-Type"))
	if guessed := normalizeMIME(mime.TypeByExtension(strings.ToLower(filepath.Ext(header.Filename)))); declared == "" || declared == "application/octet-stream" {
		declared = guessed
	}

	chosen := sniffed
	if isArchiveContainer(sniffed) && isArchiveBackedDocument(declared) {
		chosen = declared
	} else if sniffed == "application/octet-stream" {
		chosen = declared
	} else if sniffed == "text/plain" && strings.HasPrefix(declared, "text/") {
		chosen = declared
	} else if declared != "" && declared != "application/octet-stream" && topLevelMIME(sniffed) != topLevelMIME(declared) {
		return "", fmt.Errorf("file content does not match its declared type")
	}

	if chosen == "" || chosen == "application/octet-stream" {
		return "", fmt.Errorf("unknown binary file type is not allowed")
	}
	if !isAllowedMIME(chosen) {
		return "", fmt.Errorf("file type %q is not allowed", chosen)
	}
	return chosen, nil
}

func topLevelMIME(value string) string {
	if before, _, ok := strings.Cut(value, "/"); ok {
		return before
	}
	return value
}

func isArchiveContainer(value string) bool {
	return value == "application/zip" || value == "application/x-zip-compressed"
}

func isArchiveBackedDocument(value string) bool {
	return strings.HasPrefix(value, "application/vnd.openxmlformats-officedocument.") ||
		strings.HasPrefix(value, "application/vnd.oasis.opendocument.")
}

// AttachmentHandler handles file attachment API endpoints.
type AttachmentHandler struct {
	repo       *repository.AttachmentRepo
	convoRepo  *repository.ConversationRepo
	fileSvc    *filelibrary.LibraryService
	storageDir string
}

func NewAttachmentHandler(repo *repository.AttachmentRepo, convoRepo *repository.ConversationRepo, fileSvc *filelibrary.LibraryService, storageDir string) *AttachmentHandler {
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		log.Printf("[attachments] warning: could not create storage dir %s: %v", storageDir, err)
	}
	return &AttachmentHandler{repo: repo, convoRepo: convoRepo, fileSvc: fileSvc, storageDir: storageDir}
}

// Upload handles multipart file upload.
func (h *AttachmentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	conversationID := chi.URLParam(r, "conversationId")
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid upload or file too large (max %d MB)", maxUploadSize>>20))
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing 'file' field in multipart form")
		return
	}
	defer file.Close()
	if strings.TrimSpace(header.Filename) == "" || len(header.Filename) > 255 {
		respondError(w, http.StatusBadRequest, "invalid file name")
		return
	}

	mimeType, err := detectUploadedMIME(file, header)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	attachType := "file"
	if strings.HasPrefix(mimeType, "image/") {
		attachType = "image"
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if len(ext) > 16 {
		ext = ""
	}
	storageFilename := uuid.New().String() + ext
	storagePath := filepath.Join(h.storageDir, storageFilename)
	out, err := os.OpenFile(storagePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	written, copyErr := io.Copy(out, io.LimitReader(file, maxUploadSize+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || written > maxUploadSize {
		_ = os.Remove(storagePath)
		if written > maxUploadSize {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("file too large (max %d MB)", maxUploadSize>>20))
		} else {
			respondError(w, http.StatusInternalServerError, "failed to write file")
		}
		return
	}

	metadata, _ := json.Marshal(map[string]interface{}{"original_name": header.Filename})
	attachment := &models.Attachment{
		ConversationID: conversationID,
		Type:           attachType,
		MimeType:       mimeType,
		StoragePath:    storageFilename,
		Bytes:          written,
		MetadataJSON:   string(metadata),
	}
	if err := h.repo.Create(attachment); err != nil {
		_ = os.Remove(storagePath)
		respondInternalError(w, err)
		return
	}

	// Project conversations synchronously index safe, supported document files
	// into workspace scope so the next message can use them immediately.
	if attachType == "file" && h.fileSvc != nil {
		conversation, err := h.convoRepo.GetByID(conversationID)
		if err != nil {
			_ = h.repo.Delete(attachment.ID)
			_ = os.Remove(storagePath)
			respondError(w, http.StatusInternalServerError, "failed to load conversation for project indexing")
			return
		}
		if conversation != nil && conversation.WorkspaceID != nil && strings.TrimSpace(*conversation.WorkspaceID) != "" {
			userID := auth.UserIDFromContext(r.Context())
			if _, err := h.fileSvc.IngestFile(r.Context(), filelibrary.IngestFileRequest{
				OwnerUserID:    userID,
				AttachmentID:   attachment.ID,
				Scope:          "workspace",
				ConversationID: conversationID,
				WorkspaceID:    *conversation.WorkspaceID,
			}); err != nil {
				_ = h.repo.Delete(attachment.ID)
				_ = os.Remove(storagePath)
				respondError(w, http.StatusInternalServerError, "failed to index project file")
				return
			}
		}
	}
	respondJSON(w, http.StatusCreated, attachment)
}

func (h *AttachmentHandler) List(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	attachments, err := h.repo.ListByConversation(chi.URLParam(r, "conversationId"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if attachments == nil {
		attachments = []models.Attachment{}
	}
	respondJSON(w, http.StatusOK, attachments)
}

func (h *AttachmentHandler) Download(w http.ResponseWriter, r *http.Request) {
	attachment, err := h.repo.GetByID(chi.URLParam(r, "attachmentId"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if attachment == nil {
		respondError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if !verifyConversationAccessByID(w, r, h.convoRepo, attachment.ConversationID) {
		return
	}
	filePath, err := SafeJoin(h.storageDir, attachment.StoragePath)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid attachment path")
		return
	}
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			respondError(w, http.StatusNotFound, "file not found on disk")
		} else {
			respondInternalError(w, err)
		}
		return
	}
	w.Header().Set("Content-Type", attachment.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(attachment.StoragePath)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, filePath)
}

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
	if !verifyConversationAccessByID(w, r, h.convoRepo, attachment.ConversationID) {
		return
	}
	if err := h.repo.Delete(attachmentID); err != nil {
		respondInternalError(w, err)
		return
	}
	if filePath, pathErr := SafeJoin(h.storageDir, attachment.StoragePath); pathErr == nil {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			log.Printf("[attachments] warning: failed to remove file %s: %v", filePath, err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
