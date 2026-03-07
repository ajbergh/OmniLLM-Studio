package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ImageSessionHandler handles image editing session endpoints.
type ImageSessionHandler struct {
	sessionRepo *repository.ImageSessionRepo
	nodeRepo    *repository.ImageNodeRepo
	assetRepo   *repository.ImageNodeAssetRepo
	maskRepo    *repository.ImageMaskRepo
	refRepo     *repository.ImageReferenceRepo
	attachRepo  *repository.AttachmentRepo
	convoRepo   *repository.ConversationRepo
	llmSvc      *llm.Service
	storageDir  string
}

// NewImageSessionHandler creates a new ImageSessionHandler.
func NewImageSessionHandler(
	sessionRepo *repository.ImageSessionRepo,
	nodeRepo *repository.ImageNodeRepo,
	assetRepo *repository.ImageNodeAssetRepo,
	maskRepo *repository.ImageMaskRepo,
	refRepo *repository.ImageReferenceRepo,
	attachRepo *repository.AttachmentRepo,
	convoRepo *repository.ConversationRepo,
	llmSvc *llm.Service,
	storageDir string,
) *ImageSessionHandler {
	return &ImageSessionHandler{
		sessionRepo: sessionRepo,
		nodeRepo:    nodeRepo,
		assetRepo:   assetRepo,
		maskRepo:    maskRepo,
		refRepo:     refRepo,
		attachRepo:  attachRepo,
		convoRepo:   convoRepo,
		llmSvc:      llmSvc,
		storageDir:  storageDir,
	}
}

// ── Request / Response types ────────────────────────────────────────────

type createSessionRequest struct {
	Title string `json:"title,omitempty"`
}

type imageSessionGenerateRequest struct {
	Prompt  string `json:"prompt"`
	Size    string `json:"size,omitempty"`
	Quality string `json:"quality,omitempty"`
	N       int    `json:"n,omitempty"`

	Seed       *int     `json:"seed,omitempty"`
	Creativity *float64 `json:"creativity,omitempty"`

	ReferenceImageIDs []string `json:"reference_image_ids,omitempty"`
	StyleReferenceIDs []string `json:"style_reference_ids,omitempty"`

	Override *struct {
		Provider *string `json:"provider,omitempty"`
		Model    *string `json:"model,omitempty"`
	} `json:"override,omitempty"`
}

type imageSessionEditRequest struct {
	Instruction           string   `json:"instruction"`
	BaseImageAttachmentID string   `json:"base_image_attachment_id"`
	MaskAttachmentID      string   `json:"mask_attachment_id,omitempty"`
	Size                  string   `json:"size,omitempty"`
	Strength              *float64 `json:"strength,omitempty"`
	N                     int      `json:"n,omitempty"`

	ReferenceImageIDs []string `json:"reference_image_ids,omitempty"`
	StyleReferenceIDs []string `json:"style_reference_ids,omitempty"`

	Override *struct {
		Provider *string `json:"provider,omitempty"`
		Model    *string `json:"model,omitempty"`
	} `json:"override,omitempty"`
}

type generateResponse struct {
	Node   models.ImageNode        `json:"node"`
	Assets []models.ImageNodeAsset `json:"assets"`
}

type sessionDetailResponse struct {
	Session models.ImageSession `json:"session"`
	Nodes   []nodeWithMask      `json:"nodes"`
}

type nodeWithMask struct {
	models.ImageNode
	Mask *models.ImageMask `json:"mask,omitempty"`
}

// ── Handlers ─────────────────────────────────────────────────────────────

// CreateStandaloneSession creates an image session with its own backing image conversation.
// POST /v1/images/sessions
func (h *ImageSessionHandler) CreateStandaloneSession(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req createSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	title := req.Title
	if title == "" {
		title = "Untitled Session"
	}

	convo, err := h.convoRepo.CreateWithKind(userID, title, models.ConversationKindImage, nil, nil, nil)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	session, err := h.sessionRepo.Create(convo.ID, title)
	if err != nil {
		_ = h.convoRepo.Delete(convo.ID, userID)
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, session)
}

// CreateSession creates a new image editing session.
// POST /v1/conversations/{conversationId}/images/sessions
func (h *ImageSessionHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	var req createSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	title := req.Title
	if title == "" {
		title = "Untitled Session"
	}

	session, err := h.sessionRepo.Create(convoID, title)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, session)
}

// GetSession returns a session with its nodes.
// GET /v1/conversations/{conversationId}/images/sessions/{sessionId}
func (h *ImageSessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	sessionID := chi.URLParam(r, "sessionId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	session, err := h.sessionRepo.GetByID(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if session.ConversationID != convoID {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}

	nodes, err := h.nodeRepo.ListBySession(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if nodes == nil {
		nodes = []models.ImageNode{}
	}

	// Enrich nodes with their most-recent mask (if any)
	enriched := make([]nodeWithMask, len(nodes))
	for i, n := range nodes {
		enriched[i] = nodeWithMask{ImageNode: n}
		mask, err := h.maskRepo.GetByNode(n.ID)
		if err == nil && mask != nil {
			enriched[i].Mask = mask
		}
	}

	respondJSON(w, http.StatusOK, sessionDetailResponse{
		Session: *session,
		Nodes:   enriched,
	})
}

// ListAllSessions returns all image sessions across all conversations for the current user.
// GET /v1/images/sessions
func (h *ImageSessionHandler) ListAllSessions(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	sessions, err := h.sessionRepo.ListAllForUser(userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if sessions == nil {
		sessions = []models.ImageSession{}
	}
	respondJSON(w, http.StatusOK, sessions)
}

// ListSessions returns all sessions for a conversation.
// GET /v1/conversations/{conversationId}/images/sessions
func (h *ImageSessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	sessions, err := h.sessionRepo.ListByConversation(convoID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if sessions == nil {
		sessions = []models.ImageSession{}
	}
	respondJSON(w, http.StatusOK, sessions)
}

// RenameSession updates the title of an image session.
// PATCH /v1/conversations/{conversationId}/images/sessions/{sessionId}
func (h *ImageSessionHandler) RenameSession(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	sessionID := chi.URLParam(r, "sessionId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}

	if err := h.sessionRepo.UpdateTitle(sessionID, req.Title); err != nil {
		respondInternalError(w, err)
		return
	}

	sess, err := h.sessionRepo.GetByID(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, sess)
}

// DeleteSession deletes an image session and all its data.
// DELETE /v1/conversations/{conversationId}/images/sessions/{sessionId}
func (h *ImageSessionHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	sessionID := chi.URLParam(r, "sessionId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	session, err := h.sessionRepo.GetByID(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if session.ConversationID != convoID {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}

	// Clean up attachment files for all assets in this session before DB cascade delete
	nodes, _ := h.nodeRepo.ListBySession(sessionID)
	for _, n := range nodes {
		assets, _ := h.assetRepo.ListByNode(n.ID)
		for _, a := range assets {
			att, err := h.attachRepo.GetByID(a.AttachmentID)
			if err == nil && att != nil {
				safePath, pathErr := SafeJoin(h.storageDir, att.StoragePath)
				if pathErr == nil {
					os.Remove(safePath)
				}
				_ = h.attachRepo.Delete(a.AttachmentID)
			}
		}
	}

	// Clean up mask attachment files before DB cascade delete
	masks, _ := h.maskRepo.ListBySession(sessionID)
	for _, m := range masks {
		att, err := h.attachRepo.GetByID(m.AttachmentID)
		if err == nil && att != nil {
			safePath, pathErr := SafeJoin(h.storageDir, att.StoragePath)
			if pathErr == nil {
				os.Remove(safePath)
			}
			_ = h.attachRepo.Delete(m.AttachmentID)
		}
	}

	if err := h.sessionRepo.Delete(sessionID); err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// Generate creates a new image via generation and adds a node to the session.
// POST /v1/conversations/{conversationId}/images/sessions/{sessionId}/generate
func (h *ImageSessionHandler) Generate(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	sessionID := chi.URLParam(r, "sessionId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	session, err := h.sessionRepo.GetByID(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if session.ConversationID != convoID {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}

	var req imageSessionGenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Prompt == "" {
		respondError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	// Resolve provider/model
	provider, model := h.resolveProviderModel(req.Override, convo)

	// Build image request
	imgReq := llm.ImageRequest{
		Provider: provider,
		Model:    model,
		Prompt:   req.Prompt,
		Size:     req.Size,
		Quality:  req.Quality,
		N:        req.N,
	}

	// Load reference images if provided
	if len(req.ReferenceImageIDs) > 0 {
		// First content reference goes to ReferenceImage (primary, for backward compatibility)
		ref, err := h.loadReferenceImage(convoID, req.ReferenceImageIDs[0])
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		imgReq.ReferenceImage = ref

		// Additional content references
		for _, id := range req.ReferenceImageIDs[1:] {
			r, err := h.loadReferenceImage(convoID, id)
			if err != nil {
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
			imgReq.ReferenceImages = append(imgReq.ReferenceImages, *r)
		}
	}

	// Load style references if provided
	for _, id := range req.StyleReferenceIDs {
		r, err := h.loadReferenceImage(convoID, id)
		if err != nil {
			respondError(w, http.StatusBadRequest, "style ref: "+err.Error())
			return
		}
		imgReq.StyleReferenceImages = append(imgReq.StyleReferenceImages, *r)
	}

	// Call LLM service
	imgResp, err := h.llmSvc.ImageGenerate(r.Context(), imgReq)
	if err != nil {
		log.Printf("ERROR: image session generate: %v", err)
		respondError(w, http.StatusBadGateway, "image generation failed: "+err.Error())
		return
	}

	// Build params JSON
	paramsJSON := h.buildParamsJSON(req.Size, req.Quality, req.Seed, req.Creativity, nil)

	// Create node
	node := &models.ImageNode{
		SessionID:     sessionID,
		ParentNodeID:  session.ActiveNodeID,
		OperationType: "generate",
		Instruction:   req.Prompt,
		Provider:      imgResp.Provider,
		Model:         imgResp.Model,
		Seed:          req.Seed,
		ParamsJSON:    paramsJSON,
	}
	if err := h.nodeRepo.Create(node); err != nil {
		respondInternalError(w, err)
		return
	}

	// Save generated images as assets
	assets, err := h.saveImageAssets(convoID, node.ID, imgResp)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if len(assets) == 0 {
		respondError(w, http.StatusInternalServerError, "failed to save any generated images")
		return
	}

	// Save reference records
	h.saveReferenceRecords(node.ID, req.ReferenceImageIDs, "content")
	h.saveReferenceRecords(node.ID, req.StyleReferenceIDs, "style")

	// Update session active node
	if err := h.sessionRepo.UpdateActiveNode(sessionID, node.ID); err != nil {
		log.Printf("[image-session] failed to update active node: %v", err)
	}

	respondJSON(w, http.StatusCreated, generateResponse{
		Node:   *node,
		Assets: assets,
	})
}

// Edit applies an edit operation (with optional mask) to create a new node.
// POST /v1/conversations/{conversationId}/images/sessions/{sessionId}/edit
func (h *ImageSessionHandler) Edit(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	sessionID := chi.URLParam(r, "sessionId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	session, err := h.sessionRepo.GetByID(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if session.ConversationID != convoID {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}

	var req imageSessionEditRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Instruction == "" {
		respondError(w, http.StatusBadRequest, "instruction is required")
		return
	}
	if req.BaseImageAttachmentID == "" {
		respondError(w, http.StatusBadRequest, "base_image_attachment_id is required")
		return
	}

	// Load base image
	baseRef, err := h.loadReferenceImage(convoID, req.BaseImageAttachmentID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Load mask if provided
	var maskRef *llm.ReferenceImage
	if req.MaskAttachmentID != "" {
		maskRef, err = h.loadReferenceImage(convoID, req.MaskAttachmentID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "mask: "+err.Error())
			return
		}
	}

	provider, model := h.resolveProviderModel(req.Override, convo)

	// Gate: verify the provider supports editing before proceeding
	providerType, err := h.llmSvc.ResolveProviderType(provider)
	if err != nil {
		respondError(w, http.StatusBadRequest, "cannot resolve provider: "+err.Error())
		return
	}
	caps := llm.GetImageCapabilities(providerType)
	if !caps.SupportsEditing {
		respondError(w, http.StatusBadRequest, "selected provider does not support image editing")
		return
	}

	imgReq := llm.ImageRequest{
		Provider:       provider,
		Model:          model,
		Prompt:         req.Instruction,
		Size:           req.Size,
		N:              req.N,
		ReferenceImage: baseRef,
		MaskImage:      maskRef,
		OperationType:  "edit",
		Strength:       req.Strength,
	}

	// Load content references for edit
	for _, id := range req.ReferenceImageIDs {
		ref, err := h.loadReferenceImage(convoID, id)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		imgReq.ReferenceImages = append(imgReq.ReferenceImages, *ref)
	}

	// Load style references for edit
	for _, id := range req.StyleReferenceIDs {
		ref, err := h.loadReferenceImage(convoID, id)
		if err != nil {
			respondError(w, http.StatusBadRequest, "style ref: "+err.Error())
			return
		}
		imgReq.StyleReferenceImages = append(imgReq.StyleReferenceImages, *ref)
	}

	imgResp, err := h.llmSvc.ImageGenerate(r.Context(), imgReq)
	if err != nil {
		log.Printf("ERROR: image session edit: %v", err)
		respondError(w, http.StatusBadGateway, "image edit failed: "+err.Error())
		return
	}

	paramsJSON := h.buildParamsJSON(req.Size, "", nil, nil, req.Strength)

	node := &models.ImageNode{
		SessionID:     sessionID,
		ParentNodeID:  session.ActiveNodeID,
		OperationType: "edit",
		Instruction:   req.Instruction,
		Provider:      imgResp.Provider,
		Model:         imgResp.Model,
		ParamsJSON:    paramsJSON,
	}
	if err := h.nodeRepo.Create(node); err != nil {
		respondInternalError(w, err)
		return
	}

	assets, err := h.saveImageAssets(convoID, node.ID, imgResp)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if len(assets) == 0 {
		respondError(w, http.StatusInternalServerError, "failed to save any edited images")
		return
	}

	h.saveReferenceRecords(node.ID, req.ReferenceImageIDs, "content")
	h.saveReferenceRecords(node.ID, req.StyleReferenceIDs, "style")

	if err := h.sessionRepo.UpdateActiveNode(sessionID, node.ID); err != nil {
		log.Printf("[image-session] failed to update active node: %v", err)
	}

	respondJSON(w, http.StatusCreated, generateResponse{
		Node:   *node,
		Assets: assets,
	})
}

// UploadMask accepts a mask image upload.
// POST /v1/conversations/{conversationId}/images/sessions/{sessionId}/mask
func (h *ImageSessionHandler) UploadMask(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	sessionID := chi.URLParam(r, "sessionId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	session, err := h.sessionRepo.GetByID(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if session.ConversationID != convoID {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}

	var req struct {
		NodeID     string  `json:"node_id"`
		MaskData   string  `json:"mask_data"` // base64-encoded PNG
		StrokeJSON *string `json:"stroke_json,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.NodeID == "" || req.MaskData == "" {
		respondError(w, http.StatusBadRequest, "node_id and mask_data are required")
		return
	}

	// Verify the node belongs to this session
	if err := h.verifyNodeInSession(req.NodeID, sessionID); err != nil {
		respondError(w, http.StatusNotFound, "node not found in session")
		return
	}

	// Decode mask image
	maskBytes, err := base64.StdEncoding.DecodeString(req.MaskData)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid base64 mask_data")
		return
	}

	// Save mask to disk
	filename := uuid.New().String() + "_mask.png"
	filePath, pathErr := SafeJoin(h.storageDir, filename)
	if pathErr != nil {
		respondError(w, http.StatusBadRequest, "invalid storage path")
		return
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		respondInternalError(w, err)
		return
	}
	if err := os.WriteFile(filePath, maskBytes, 0o644); err != nil {
		respondInternalError(w, err)
		return
	}

	// Create attachment record
	attachment := &models.Attachment{
		ConversationID: convoID,
		Type:           "image",
		MimeType:       "image/png",
		StoragePath:    filename,
		Bytes:          int64(len(maskBytes)),
		CreatedAt:      time.Now().UTC(),
		MetadataJSON:   `{"type":"mask"}`,
	}
	if err := h.attachRepo.Create(attachment); err != nil {
		respondInternalError(w, err)
		return
	}

	// Create mask record
	mask := &models.ImageMask{
		NodeID:       req.NodeID,
		AttachmentID: attachment.ID,
		StrokeJSON:   req.StrokeJSON,
	}
	if err := h.maskRepo.Create(mask); err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"mask_id":       mask.ID,
		"attachment_id": attachment.ID,
	})
}

// GetAssets returns all assets for a session's active node.
// GET /v1/conversations/{conversationId}/images/sessions/{sessionId}/assets
func (h *ImageSessionHandler) GetAssets(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	sessionID := chi.URLParam(r, "sessionId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	session, err := h.sessionRepo.GetByID(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if session.ConversationID != convoID {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}

	// If scope=session, return all assets for the session with optional type filter
	if r.URL.Query().Get("scope") == "session" {
		opTypes := r.URL.Query()["type"]
		sortDesc := r.URL.Query().Get("sort") != "created_at_asc"
		assets, err := h.assetRepo.ListBySession(sessionID, opTypes, sortDesc)
		if err != nil {
			respondInternalError(w, err)
			return
		}
		if assets == nil {
			assets = []models.ImageNodeAsset{}
		}
		respondJSON(w, http.StatusOK, assets)
		return
	}

	// Default: get assets for a specific node
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" && session.ActiveNodeID != nil {
		nodeID = *session.ActiveNodeID
	}
	if nodeID == "" {
		respondJSON(w, http.StatusOK, []models.ImageNodeAsset{})
		return
	}

	// Verify the node belongs to this session
	if err := h.verifyNodeInSession(nodeID, sessionID); err != nil {
		respondError(w, http.StatusNotFound, "node not found in session")
		return
	}

	assets, err := h.assetRepo.ListByNode(nodeID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if assets == nil {
		assets = []models.ImageNodeAsset{}
	}
	respondJSON(w, http.StatusOK, assets)
}

// DeleteAsset removes a single asset.
// DELETE /v1/conversations/{conversationId}/images/sessions/{sessionId}/assets/{assetId}
func (h *ImageSessionHandler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	sessionID := chi.URLParam(r, "sessionId")
	assetID := chi.URLParam(r, "assetId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	session, err := h.sessionRepo.GetByID(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if session.ConversationID != convoID {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}

	// Verify the asset belongs to this session
	if err := h.verifyAssetInSession(assetID, sessionID); err != nil {
		respondError(w, http.StatusNotFound, "asset not found in session")
		return
	}

	if err := h.assetRepo.Delete(assetID); err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// SelectVariant sets the selected variant for a node.
// PUT /v1/conversations/{conversationId}/images/sessions/{sessionId}/nodes/{nodeId}/select
func (h *ImageSessionHandler) SelectVariant(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	sessionID := chi.URLParam(r, "sessionId")
	nodeID := chi.URLParam(r, "nodeId")
	userID := auth.UserIDFromContext(r.Context())

	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	session, err := h.sessionRepo.GetByID(sessionID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if session.ConversationID != convoID {
		respondError(w, http.StatusNotFound, "session not found")
		return
	}

	var req struct {
		AssetID string `json:"asset_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AssetID == "" {
		respondError(w, http.StatusBadRequest, "asset_id is required")
		return
	}

	// Verify both the node and the asset belong to this session
	if err := h.verifyNodeInSession(nodeID, sessionID); err != nil {
		respondError(w, http.StatusNotFound, "node not found in session")
		return
	}
	if err := h.verifyAssetInSession(req.AssetID, sessionID); err != nil {
		respondError(w, http.StatusNotFound, "asset not found in session")
		return
	}

	if err := h.assetRepo.SetSelected(nodeID, req.AssetID); err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ── Helpers ──────────────────────────────────────────────────────────────

func (h *ImageSessionHandler) resolveProviderModel(override *struct {
	Provider *string `json:"provider,omitempty"`
	Model    *string `json:"model,omitempty"`
}, convo *models.Conversation) (string, string) {
	provider := ""
	model := ""
	if override != nil {
		if override.Provider != nil {
			provider = *override.Provider
		}
		if override.Model != nil {
			model = *override.Model
		}
	}
	if provider == "" && convo.DefaultProvider != nil {
		provider = *convo.DefaultProvider
	}
	return provider, model
}

// verifyNodeInSession checks that a node belongs to the given session.
func (h *ImageSessionHandler) verifyNodeInSession(nodeID, sessionID string) error {
	node, err := h.nodeRepo.GetByID(nodeID)
	if err != nil {
		return fmt.Errorf("node not found in session")
	}
	if node == nil || node.SessionID != sessionID {
		return fmt.Errorf("node not found in session")
	}
	return nil
}

// verifyAssetInSession checks that an asset belongs to a node within the given session.
func (h *ImageSessionHandler) verifyAssetInSession(assetID, sessionID string) error {
	asset, err := h.assetRepo.GetByID(assetID)
	if err != nil || asset == nil {
		return fmt.Errorf("asset not found in session")
	}
	return h.verifyNodeInSession(asset.NodeID, sessionID)
}

func (h *ImageSessionHandler) loadReferenceImage(convoID, attachmentID string) (*llm.ReferenceImage, error) {
	att, err := h.attachRepo.GetByID(attachmentID)
	if err != nil {
		return nil, fmt.Errorf("load reference image: %w", err)
	}
	if att == nil {
		return nil, fmt.Errorf("attachment %s not found", attachmentID)
	}
	if att.ConversationID != convoID {
		return nil, fmt.Errorf("attachment does not belong to this conversation")
	}
	if len(att.MimeType) < 6 || att.MimeType[:6] != "image/" {
		return nil, fmt.Errorf("attachment %s is not an image (type: %s)", attachmentID, att.MimeType)
	}
	refPath, pathErr := SafeJoin(h.storageDir, att.StoragePath)
	if pathErr != nil {
		return nil, fmt.Errorf("invalid attachment path")
	}
	imgBytes, err := os.ReadFile(refPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}
	return &llm.ReferenceImage{
		Data:     base64.StdEncoding.EncodeToString(imgBytes),
		MimeType: att.MimeType,
	}, nil
}

func (h *ImageSessionHandler) saveImageAssets(convoID, nodeID string, imgResp *llm.ImageResponse) ([]models.ImageNodeAsset, error) {
	var assets []models.ImageNodeAsset

	for i, img := range imgResp.Images {
		imgData, err := base64.StdEncoding.DecodeString(img.B64JSON)
		if err != nil {
			log.Printf("[image-session] failed to decode base64 for image %d: %v", i, err)
			continue
		}

		filename := uuid.New().String() + ".png"
		filePath, pathErr := SafeJoin(h.storageDir, filename)
		if pathErr != nil {
			log.Printf("[image-session] invalid path for image %d: %v", i, pathErr)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			log.Printf("[image-session] mkdir failed: %v", err)
			continue
		}
		if err := os.WriteFile(filePath, imgData, 0o644); err != nil {
			log.Printf("[image-session] failed to write image %d: %v", i, err)
			continue
		}

		metaMap := map[string]string{
			"revised_prompt":  img.RevisedPrompt,
			"generator_model": imgResp.Model,
		}
		metaBytes, _ := json.Marshal(metaMap)

		attachment := &models.Attachment{
			ConversationID: convoID,
			Type:           "image",
			MimeType:       "image/png",
			StoragePath:    filename,
			Bytes:          int64(len(imgData)),
			CreatedAt:      time.Now().UTC(),
			MetadataJSON:   string(metaBytes),
		}
		if err := h.attachRepo.Create(attachment); err != nil {
			log.Printf("[image-session] failed to create attachment: %v", err)
			continue
		}

		asset := &models.ImageNodeAsset{
			NodeID:       nodeID,
			AttachmentID: attachment.ID,
			VariantIndex: i,
			IsSelected:   i == 0,
		}
		if err := h.assetRepo.Create(asset); err != nil {
			log.Printf("[image-session] failed to create asset record: %v", err)
			continue
		}
		assets = append(assets, *asset)
	}
	return assets, nil
}

func (h *ImageSessionHandler) saveReferenceRecords(nodeID string, attachmentIDs []string, role string) {
	for i, id := range attachmentIDs {
		ref := &models.ImageReference{
			NodeID:       nodeID,
			AttachmentID: id,
			RefRole:      role,
			SortOrder:    i,
		}
		if err := h.refRepo.Create(ref); err != nil {
			log.Printf("[image-session] failed to save reference record: %v", err)
		}
	}
}

func (h *ImageSessionHandler) buildParamsJSON(size, quality string, seed *int, creativity *float64, strength *float64) *string {
	params := map[string]interface{}{}
	if size != "" {
		params["size"] = size
	}
	if quality != "" {
		params["quality"] = quality
	}
	if seed != nil {
		params["seed"] = *seed
	}
	if creativity != nil {
		params["creativity"] = *creativity
	}
	if strength != nil {
		params["strength"] = *strength
	}
	if len(params) == 0 {
		return nil
	}
	b, _ := json.Marshal(params)
	s := string(b)
	return &s
}
