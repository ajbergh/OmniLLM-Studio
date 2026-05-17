// Package api provides HTTP handlers and routing for the OmniLLM-Studio backend.
// This file contains the handlers for managing Model Context Protocol (MCP) servers.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/mcpclient"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// MCPHandler exposes MCP server management endpoints.
type MCPHandler struct {
	repo    *repository.MCPServerRepo
	manager *mcpclient.Manager
}

// NewMCPHandler creates an MCPHandler.
func NewMCPHandler(repo *repository.MCPServerRepo, manager *mcpclient.Manager) *MCPHandler {
	return &MCPHandler{repo: repo, manager: manager}
}

func (h *MCPHandler) ListServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.manager.ListServers()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, servers)
}

func (h *MCPHandler) ListAudit(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 {
			respondError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsed
	}

	events, err := h.repo.ListAudit(repository.MCPAuditFilter{
		ServerID: r.URL.Query().Get("server_id"),
		Limit:    limit,
	})
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, events)
}

func (h *MCPHandler) GetServer(w http.ResponseWriter, r *http.Request) {
	server, err := h.manager.GetServer(chi.URLParam(r, "serverId"))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if server == nil {
		respondError(w, http.StatusNotFound, "MCP server not found")
		return
	}
	respondJSON(w, http.StatusOK, server)
}

func (h *MCPHandler) CreateServer(w http.ResponseWriter, r *http.Request) {
	var input repository.CreateMCPServerInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateCreateMCPServer(input); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	server, err := h.repo.Create(input)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	h.auditConfigChange(server.ID, "server_create", nil)
	if server.Enabled {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		if err := h.manager.Start(ctx, server.ID); err != nil {
			cancel()
			enriched, _ := h.manager.GetServer(server.ID)
			respondJSON(w, http.StatusCreated, enriched)
			return
		}
		cancel()
	}

	enriched, err := h.manager.GetServer(server.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, enriched)
}

func (h *MCPHandler) UpdateServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "serverId")
	var input repository.UpdateMCPServerInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateUpdateMCPServer(input); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	_ = h.manager.Stop(r.Context(), id)

	server, err := h.repo.Update(id, input)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if server == nil {
		respondError(w, http.StatusNotFound, "MCP server not found")
		return
	}
	h.auditConfigChange(server.ID, "server_update", nil)
	if server.Enabled {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		if err := h.manager.Start(ctx, server.ID); err != nil {
			cancel()
			enriched, _ := h.manager.GetServer(server.ID)
			respondJSON(w, http.StatusOK, enriched)
			return
		}
		cancel()
	}

	enriched, err := h.manager.GetServer(server.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, enriched)
}

func (h *MCPHandler) DeleteServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "serverId")
	_ = h.manager.Stop(r.Context(), id)
	if err := h.repo.Delete(id); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "MCP server not found")
			return
		}
		respondInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MCPHandler) StartServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "serverId")
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.manager.Start(ctx, id); err != nil {
		respondMCPRuntimeError(w, err)
		return
	}
	h.respondServer(w, id)
}

func (h *MCPHandler) StopServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "serverId")
	if err := h.manager.Stop(r.Context(), id); err != nil {
		respondMCPRuntimeError(w, err)
		return
	}
	h.respondServer(w, id)
}

func (h *MCPHandler) RestartServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "serverId")
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.manager.Restart(ctx, id); err != nil {
		respondMCPRuntimeError(w, err)
		return
	}
	h.respondServer(w, id)
}

func (h *MCPHandler) RefreshTools(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "serverId")
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	toolList, err := h.manager.RefreshTools(ctx, id)
	if err != nil {
		respondMCPRuntimeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, toolList)
}

func (h *MCPHandler) TestServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "serverId")
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	toolList, err := h.manager.Test(ctx, id)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if err == sql.ErrNoRows {
			status = http.StatusNotFound
		}
		respondJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"tools": toolList,
	})
}

func (h *MCPHandler) ListTools(w http.ResponseWriter, r *http.Request) {
	toolList, err := h.manager.ListTools(chi.URLParam(r, "serverId"))
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "MCP server not found")
			return
		}
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, toolList)
}

func (h *MCPHandler) UpdateToolPolicy(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverId")
	internalName := chi.URLParam(r, "toolName")

	var req struct {
		Policy string `json:"policy"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.manager.SetToolPolicy(serverID, internalName, req.Policy); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "MCP tool not found")
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload, _ := json.Marshal(map[string]string{"policy": req.Policy})
	inputJSON := string(payload)
	h.auditConfigChange(serverID, "policy_change", &modelsAuditPayload{
		toolName:  internalName,
		inputJSON: inputJSON,
	})
	respondJSON(w, http.StatusOK, map[string]string{
		"tool_name": internalName,
		"policy":    req.Policy,
	})
}

func (h *MCPHandler) respondServer(w http.ResponseWriter, id string) {
	server, err := h.manager.GetServer(id)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if server == nil {
		respondError(w, http.StatusNotFound, "MCP server not found")
		return
	}
	respondJSON(w, http.StatusOK, server)
}

type modelsAuditPayload struct {
	toolName  string
	inputJSON string
}

func (h *MCPHandler) auditConfigChange(serverID, eventType string, payload *modelsAuditPayload) {
	event := repositoryMCPAuditEvent(serverID, eventType, payload)
	if err := h.repo.InsertAudit(event); err != nil {
		// Audit logging should not make the admin action fail.
		return
	}
}

func repositoryMCPAuditEvent(serverID, eventType string, payload *modelsAuditPayload) models.MCPAuditEvent {
	event := models.MCPAuditEvent{
		ServerID:  serverID,
		EventType: eventType,
	}
	if payload != nil {
		event.ToolName = &payload.toolName
		event.InputJSON = &payload.inputJSON
	}
	return event
}

func validateCreateMCPServer(input repository.CreateMCPServerInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return errString("name is required")
	}
	transport := strings.TrimSpace(input.Transport)
	if transport == "" {
		transport = "stdio"
	}
	switch transport {
	case "stdio":
		if input.Command == nil || strings.TrimSpace(*input.Command) == "" {
			return errString("command is required for stdio transport")
		}
	case "http":
		if input.URL == nil || strings.TrimSpace(*input.URL) == "" {
			return errString("url is required for http transport")
		}
	default:
		return errString("transport must be stdio or http")
	}
	return nil
}

func validateUpdateMCPServer(input repository.UpdateMCPServerInput) error {
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		return errString("name cannot be empty")
	}
	if input.Transport != nil {
		transport := strings.TrimSpace(*input.Transport)
		if transport != "stdio" && transport != "http" {
			return errString("transport must be stdio or http")
		}
	}
	if input.Command != nil && strings.TrimSpace(*input.Command) == "" {
		return errString("command cannot be empty")
	}
	if input.URL != nil && strings.TrimSpace(*input.URL) == "" {
		return errString("url cannot be empty")
	}
	return nil
}

func respondMCPRuntimeError(w http.ResponseWriter, err error) {
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "MCP server not found")
		return
	}
	respondJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
}

type errString string

func (e errString) Error() string { return string(e) }
