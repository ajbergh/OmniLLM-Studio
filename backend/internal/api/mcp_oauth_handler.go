package api

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/mcpclient"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/go-chi/chi/v5"
)

// MCPOAuthHandler exposes admin configuration and browser authorization flows.
type MCPOAuthHandler struct {
	oauth   *mcpclient.OAuthService
	manager *mcpclient.Manager
}

func NewMCPOAuthHandler(oauth *mcpclient.OAuthService, manager *mcpclient.Manager) *MCPOAuthHandler {
	return &MCPOAuthHandler{oauth: oauth, manager: manager}
}

func (h *MCPOAuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	status, err := h.oauth.Status(chi.URLParam(r, "id"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func (h *MCPOAuthHandler) Configure(w http.ResponseWriter, r *http.Request) {
	var input models.ConfigureMCPOAuthInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid OAuth configuration")
		return
	}
	if err := h.oauth.Configure(chi.URLParam(r, "id"), input); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	status, err := h.oauth.Status(chi.URLParam(r, "id"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func (h *MCPOAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	start, err := h.oauth.StartAuthorization(r.Context(), chi.URLParam(r, "id"), auth.ScopeUserIDFromContext(r.Context()))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, start)
}

func (h *MCPOAuthHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	if err := h.oauth.Disconnect(serverID); err != nil {
		respondInternalError(w, err)
		return
	}
	stopCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_ = h.manager.Stop(stopCtx, serverID)
	w.WriteHeader(http.StatusNoContent)
}

// Callback intentionally lives outside the authenticated admin route group: the
// authorization server redirects the user's browser here. One-time state binds
// the response to the originating authenticated admin action.
func (h *MCPOAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	issuer := strings.TrimSpace(r.URL.Query().Get("iss"))
	if oauthError := strings.TrimSpace(r.URL.Query().Get("error")); oauthError != "" {
		if err := h.oauth.RejectAuthorization(state, issuer); err != nil {
			h.writeCallbackPage(w, false, "Authorization response validation failed.")
			return
		}
		description := strings.TrimSpace(r.URL.Query().Get("error_description"))
		if description == "" {
			description = oauthError
		}
		h.writeCallbackPage(w, false, "Authorization was not completed: "+description)
		return
	}
	serverID, err := h.oauth.CompleteAuthorization(r.Context(), state, r.URL.Query().Get("code"), issuer)
	if err != nil {
		h.writeCallbackPage(w, false, err.Error())
		return
	}

	restartCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	restartErr := h.manager.Restart(restartCtx, serverID)
	cancel()
	if restartErr != nil {
		h.writeCallbackPage(w, true, "Authorization succeeded. The MCP server saved the token but could not reconnect automatically; return to Settings and retry the connection.")
		return
	}
	h.writeCallbackPage(w, true, "Authorization succeeded. You can close this window and return to OmniLLM-Studio.")
}

func (h *MCPOAuthHandler) writeCallbackPage(w http.ResponseWriter, success bool, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	status := "MCP authorization failed"
	if success {
		status = "MCP authorization complete"
	}
	_, _ = fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>%s</title></head><body><main style="font-family:system-ui;max-width:640px;margin:64px auto;padding:0 24px"><h1>%s</h1><p>%s</p></main></body></html>`, html.EscapeString(status), html.EscapeString(status), html.EscapeString(message))
}
