package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/music"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

type MusicHandler struct {
	service     *music.Service
	sessionRepo *repository.MusicSessionRepo
	genRepo     *repository.MusicGenerationRepo
	assetRepo   *repository.MusicAssetRepo
	storageDir  string
}

func NewMusicHandler(
	service *music.Service,
	sessionRepo *repository.MusicSessionRepo,
	genRepo *repository.MusicGenerationRepo,
	assetRepo *repository.MusicAssetRepo,
	storageDir string,
) *MusicHandler {
	return &MusicHandler{
		service:     service,
		sessionRepo: sessionRepo,
		genRepo:     genRepo,
		assetRepo:   assetRepo,
		storageDir:  storageDir,
	}
}

func (h *MusicHandler) Providers(w http.ResponseWriter, r *http.Request) {
	providers, err := h.service.ListProviders()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, providers)
}

func (h *MusicHandler) Models(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	models, err := h.service.ListModels(r.Context(), provider, false)
	if err != nil && len(models) == 0 {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if models == nil {
		models = []music.Model{}
	}
	respondJSON(w, http.StatusOK, models)
}

func (h *MusicHandler) RefreshModels(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	models, err := h.service.ListModels(r.Context(), provider, true)
	if err != nil && len(models) == 0 {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if models == nil {
		models = []music.Model{}
	}
	respondJSON(w, http.StatusOK, models)
}

func (h *MusicHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.sessionRepo.ListForUser(auth.UserIDFromContext(r.Context()))
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if sessions == nil {
		sessions = []models.MusicSession{}
	}
	respondJSON(w, http.StatusOK, sessions)
}

func (h *MusicHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string `json:"title,omitempty"`
		Provider string `json:"provider,omitempty"`
		Model    string `json:"model,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	session, err := h.service.CreateSession(auth.UserIDFromContext(r.Context()), req.Title, req.Provider, req.Model)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, session)
}

func (h *MusicHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	session, ok := h.loadOwnedSession(w, r, chi.URLParam(r, "sessionId"))
	if !ok {
		return
	}
	generations, err := h.genRepo.ListBySession(session.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"session":     session,
		"generations": h.enrichGenerations(generations),
	})
}

func (h *MusicHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	session, ok := h.loadOwnedSession(w, r, chi.URLParam(r, "sessionId"))
	if !ok {
		return
	}
	var req struct {
		Title    string `json:"title,omitempty"`
		Provider string `json:"provider,omitempty"`
		Model    string `json:"model,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := h.sessionRepo.Update(session.ID, req.Title, req.Provider, req.Model)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

func (h *MusicHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	session, ok := h.loadOwnedSession(w, r, chi.URLParam(r, "sessionId"))
	if !ok {
		return
	}
	assets, _ := h.assetRepo.ListBySession(session.ID)
	for _, asset := range assets {
		if fullPath, err := SafeJoin(h.storageDir, asset.FilePath); err == nil {
			_ = os.Remove(fullPath)
		}
	}
	if err := h.sessionRepo.Delete(session.ID); err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *MusicHandler) ListGenerations(w http.ResponseWriter, r *http.Request) {
	session, ok := h.loadOwnedSession(w, r, chi.URLParam(r, "sessionId"))
	if !ok {
		return
	}
	generations, err := h.genRepo.ListBySession(session.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, h.enrichGenerations(generations))
}

func (h *MusicHandler) GetGeneration(w http.ResponseWriter, r *http.Request) {
	gen, ok := h.loadOwnedGeneration(w, r, chi.URLParam(r, "generationId"))
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, h.enrichGeneration(*gen))
}

func (h *MusicHandler) BranchGeneration(w http.ResponseWriter, r *http.Request) {
	gen, ok := h.loadOwnedGeneration(w, r, chi.URLParam(r, "generationId"))
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"parent_id":        gen.ID,
		"session_id":       gen.SessionID,
		"prompt":           gen.Prompt,
		"assembled_prompt": gen.AssembledPrompt,
		"provider":         gen.Provider,
		"model":            gen.Model,
	})
}

func (h *MusicHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var req music.GenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	writeEvent := func(event string, payload interface{}) {
		data, _ := json.Marshal(payload)
		_, _ = w.Write([]byte("event: " + event + "\n"))
		_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		flusher.Flush()
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-done:
				return
			case <-ticker.C:
				writeEvent("music_generation_progress", map[string]string{"stage": "waiting", "message": "Still generating audio"})
			}
		}
	}()

	session, generation, asset, err := h.service.Generate(r.Context(), auth.UserIDFromContext(r.Context()), req, func(progress music.GenerationProgress) {
		if progress.Stage == "started" {
			writeEvent("music_generation_started", progress)
			return
		}
		writeEvent("music_generation_progress", progress)
	})
	close(done)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, music.ErrCapabilityUnsupported) || errors.Is(err, music.ErrProviderUnavailable) {
			status = http.StatusBadRequest
		}
		writeEvent("music_generation_error", map[string]interface{}{
			"error":         err.Error(),
			"status":        status,
			"session_id":    idOrEmpty(session),
			"generation_id": generationIDOrEmpty(generation),
		})
		return
	}
	writeEvent("music_generation_done", map[string]interface{}{
		"session":    session,
		"generation": h.enrichGeneration(*generation),
		"asset":      asset,
	})
}

func (h *MusicHandler) GetAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := h.loadOwnedAsset(w, r, chi.URLParam(r, "assetId"))
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, asset)
}

func (h *MusicHandler) DownloadAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := h.loadOwnedAsset(w, r, chi.URLParam(r, "assetId"))
	if !ok {
		return
	}
	fullPath, err := SafeJoin(h.storageDir, asset.FilePath)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid asset path")
		return
	}
	w.Header().Set("Content-Type", asset.MimeType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(asset.FileName)+`"`)
	http.ServeFile(w, r, fullPath)
}

func (h *MusicHandler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	asset, ok := h.loadOwnedAsset(w, r, chi.URLParam(r, "assetId"))
	if !ok {
		return
	}
	if fullPath, err := SafeJoin(h.storageDir, asset.FilePath); err == nil {
		_ = os.Remove(fullPath)
	}
	if err := h.assetRepo.Delete(asset.ID); err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *MusicHandler) loadOwnedSession(w http.ResponseWriter, r *http.Request, sessionID string) (*models.MusicSession, bool) {
	session, err := h.sessionRepo.GetByID(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return nil, false
	}
	if session == nil {
		respondError(w, http.StatusNotFound, "music session not found")
		return nil, false
	}
	if userID := auth.UserIDFromContext(r.Context()); userID != "" && (session.UserID == nil || *session.UserID != userID) {
		respondError(w, http.StatusNotFound, "music session not found")
		return nil, false
	}
	return session, true
}

func (h *MusicHandler) loadOwnedGeneration(w http.ResponseWriter, r *http.Request, generationID string) (*models.MusicGeneration, bool) {
	gen, err := h.genRepo.GetByID(generationID)
	if err != nil {
		respondInternalError(w, err)
		return nil, false
	}
	if gen == nil {
		respondError(w, http.StatusNotFound, "music generation not found")
		return nil, false
	}
	if _, ok := h.loadOwnedSession(w, r, gen.SessionID); !ok {
		return nil, false
	}
	return gen, true
}

func (h *MusicHandler) loadOwnedAsset(w http.ResponseWriter, r *http.Request, assetID string) (*models.MusicAsset, bool) {
	asset, err := h.assetRepo.GetByID(assetID)
	if err != nil {
		respondInternalError(w, err)
		return nil, false
	}
	if asset == nil {
		respondError(w, http.StatusNotFound, "music asset not found")
		return nil, false
	}
	if _, ok := h.loadOwnedSession(w, r, asset.SessionID); !ok {
		return nil, false
	}
	return asset, true
}

func (h *MusicHandler) enrichGenerations(generations []models.MusicGeneration) []music.GenerationDetail {
	out := make([]music.GenerationDetail, 0, len(generations))
	for _, gen := range generations {
		out = append(out, h.enrichGeneration(gen))
	}
	return out
}

func (h *MusicHandler) enrichGeneration(gen models.MusicGeneration) music.GenerationDetail {
	detail := music.GenerationDetail{
		ID:              gen.ID,
		SessionID:       gen.SessionID,
		ParentID:        gen.ParentID,
		Title:           gen.Title,
		Status:          gen.Status,
		Provider:        gen.Provider,
		Model:           gen.Model,
		Prompt:          gen.Prompt,
		AssembledPrompt: gen.AssembledPrompt,
		Lyrics:          gen.Lyrics,
		Structure:       gen.Structure,
		Error:           gen.Error,
		CostUSD:         gen.CostUSD,
		DurationMS:      gen.DurationMS,
		OutputBytes:     gen.OutputBytes,
		CreatedAt:       gen.CreatedAt,
		CompletedAt:     gen.CompletedAt,
	}
	if asset, err := h.assetRepo.GetByGeneration(gen.ID); err == nil && asset != nil {
		detail.AssetID = asset.ID
		detail.AssetURL = "/v1/music/assets/" + asset.ID + "/download"
		detail.MimeType = asset.MimeType
	}
	return detail
}

func idOrEmpty(session *models.MusicSession) string {
	if session == nil {
		return ""
	}
	return session.ID
}

func generationIDOrEmpty(generation *models.MusicGeneration) string {
	if generation == nil {
		return ""
	}
	return generation.ID
}
