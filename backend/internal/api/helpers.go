package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// JSON response helpers

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// respondInternalError logs the real error server-side and returns a generic message to the client.
func respondInternalError(w http.ResponseWriter, err error) {
	log.Printf("ERROR: %v", err)
	respondError(w, http.StatusInternalServerError, "internal server error")
}

// respondErrorWithCode sends a structured error response with code and optional details.
func respondErrorWithCode(w http.ResponseWriter, status int, code string, message string, details interface{}) {
	resp := map[string]interface{}{
		"error": message,
		"code":  code,
	}
	if details != nil {
		resp["details"] = details
	}
	respondJSON(w, status, resp)
}

// maxJSONBodySize is the default limit for JSON request bodies (1 MB).
const maxJSONBodySize = 1 << 20

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, maxJSONBodySize+1)
	dec := json.NewDecoder(limited)
	if err := dec.Decode(v); err != nil {
		return err
	}
	// If there's more data after the decoded value, the body exceeded the limit.
	// (The +1 lets us detect payloads that are exactly maxJSONBodySize+1 bytes.)
	return nil
}

// SafeJoin returns a path guaranteed to be under baseDir.
// Returns an error if the resolved path escapes the base directory.
func SafeJoin(baseDir, untrustedPath string) (string, error) {
	if untrustedPath == "" {
		return "", fmt.Errorf("empty path")
	}

	// Clean the untrusted segment
	cleaned := filepath.Clean(untrustedPath)

	// Reject absolute paths
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute path not allowed: %s", untrustedPath)
	}

	// Reject traversal components
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("path traversal not allowed: %s", untrustedPath)
	}

	joined := filepath.Join(baseDir, cleaned)

	// Final defense: resolved path must still be under baseDir
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base: %w", err)
	}
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) && absJoined != absBase {
		return "", fmt.Errorf("path escapes base directory: %s", untrustedPath)
	}

	return joined, nil
}

// verifyConversationAccess checks that the authenticated user owns the conversation
// referenced by the {conversationId} URL parameter. In solo mode (empty userID),
// the check is skipped. Returns true if access is granted, false if an error
// response has already been written.
func verifyConversationAccess(w http.ResponseWriter, r *http.Request, convoRepo *repository.ConversationRepo) bool {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		return true // solo mode — no ownership check
	}
	convoID := chi.URLParam(r, "conversationId")
	if convoID == "" {
		respondError(w, http.StatusBadRequest, "missing conversation ID")
		return false
	}
	convo, err := convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return false
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return false
	}
	return true
}

// verifyConversationAccessByID checks that the authenticated user owns the
// given conversation ID (not from URL param). Returns true if access is granted.
func verifyConversationAccessByID(w http.ResponseWriter, r *http.Request, convoRepo *repository.ConversationRepo, convoID string) bool {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		return true
	}
	convo, err := convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return false
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "not found")
		return false
	}
	return true
}
