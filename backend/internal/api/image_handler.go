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

// ImageHandler handles image generation endpoints.
type ImageHandler struct {
	msgRepo    *repository.MessageRepo
	convoRepo  *repository.ConversationRepo
	attachRepo *repository.AttachmentRepo
	llmSvc     *llm.Service
	storageDir string
}

// NewImageHandler creates a new ImageHandler.
func NewImageHandler(
	msgRepo *repository.MessageRepo,
	convoRepo *repository.ConversationRepo,
	attachRepo *repository.AttachmentRepo,
	llmSvc *llm.Service,
	storageDir string,
) *ImageHandler {
	return &ImageHandler{
		msgRepo:    msgRepo,
		convoRepo:  convoRepo,
		attachRepo: attachRepo,
		llmSvc:     llmSvc,
		storageDir: storageDir,
	}
}

type imageGenerateRequest struct {
	Prompt  string `json:"prompt"`
	Size    string `json:"size,omitempty"`
	Quality string `json:"quality,omitempty"`
	N       int    `json:"n,omitempty"`

	// ReferenceImageID is the attachment ID of a previously generated image
	// to use as the starting point for editing.
	ReferenceImageID string `json:"reference_image_id,omitempty"`

	Override *struct {
		Provider *string `json:"provider,omitempty"`
		Model    *string `json:"model,omitempty"`
	} `json:"override,omitempty"`
}

type imageGenerateResponse struct {
	UserMessage      models.Message      `json:"user_message"`
	AssistantMessage models.Message      `json:"assistant_message"`
	Attachments      []models.Attachment `json:"attachments"`
}

// Generate handles image generation for a conversation.
// POST /v1/conversations/{conversationId}/messages/image
func (h *ImageHandler) Generate(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	userID := auth.UserIDFromContext(r.Context())

	var req imageGenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Prompt == "" {
		respondError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	// Load conversation (ownership check)
	convo, err := h.convoRepo.GetByIDForUser(convoID, userID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if convo == nil {
		respondError(w, http.StatusNotFound, "conversation not found")
		return
	}

	// Save user message (the prompt)
	userMsg := &models.Message{
		ID:             uuid.New().String(),
		ConversationID: convoID,
		Role:           "user",
		Content:        req.Prompt,
		CreatedAt:      time.Now().UTC(),
		MetadataJSON:   `{"type":"image_prompt"}`,
	}
	if _, err := h.msgRepo.Create(userMsg); err != nil {
		respondInternalError(w, err)
		return
	}

	// Resolve provider/model overrides
	provider := ""
	model := ""
	if req.Override != nil {
		if req.Override.Provider != nil {
			provider = *req.Override.Provider
		}
		if req.Override.Model != nil {
			model = *req.Override.Model
		}
	}
	// Only fall back to conversation provider — never fall back to the
	// conversation's chat model, because chat models (e.g. gpt-4o) are
	// wrong for image generation. The LLM service resolves the correct
	// image model default for the provider.
	if provider == "" && convo.DefaultProvider != nil {
		provider = *convo.DefaultProvider
	}

	// Call image generation
	start := time.Now()
	imgReq := llm.ImageRequest{
		Provider: provider,
		Model:    model,
		Prompt:   req.Prompt,
		Size:     req.Size,
		Quality:  req.Quality,
		N:        req.N,
	}

	// If a reference image is provided, load it from disk for editing
	if req.ReferenceImageID != "" {
		refAttach, err := h.attachRepo.GetByID(req.ReferenceImageID)
		if err != nil {
			respondInternalError(w, err)
			return
		}
		if refAttach == nil {
			respondError(w, http.StatusNotFound, "reference image attachment not found")
			return
		}
		// Verify ownership — attachment must belong to this conversation
		if refAttach.ConversationID != convoID {
			respondError(w, http.StatusForbidden, "reference image does not belong to this conversation")
			return
		}

		refPath, pathErr := SafeJoin(h.storageDir, refAttach.StoragePath)
		if pathErr != nil {
			respondError(w, http.StatusBadRequest, "invalid reference image path")
			return
		}
		imgBytes, err := os.ReadFile(refPath)
		if err != nil {
			log.Printf("[image] failed to read reference image %s: %v", refAttach.ID, err)
			respondError(w, http.StatusInternalServerError, "failed to read reference image")
			return
		}
		imgReq.ReferenceImage = &llm.ReferenceImage{
			Data:     base64.StdEncoding.EncodeToString(imgBytes),
			MimeType: refAttach.MimeType,
		}
		log.Printf("[image] using reference image %s (%s, %d bytes) for editing", refAttach.ID, refAttach.MimeType, len(imgBytes))
	}

	imgResp, err := h.llmSvc.ImageGenerate(r.Context(), imgReq)
	if err != nil {
		log.Printf("ERROR: image generation: %v", err)
		// Clean up the user message we stored before the API call
		_ = h.msgRepo.Delete(convoID, userMsg.ID)
		respondError(w, http.StatusBadGateway, "image generation failed: "+err.Error())
		return
	}
	latency := int(time.Since(start).Milliseconds())

	// Save each generated image as an attachment
	var attachments []models.Attachment
	var imageMarkdownParts []string
	var failedImages int

	for i, img := range imgResp.Images {
		imgData, err := base64.StdEncoding.DecodeString(img.B64JSON)
		if err != nil {
			log.Printf("[image] failed to decode base64 for image %d: %v", i, err)
			failedImages++
			continue
		}

		// Save to disk
		filename := uuid.New().String() + ".png"
		filePath := filepath.Join(h.storageDir, filename)
		if err := os.WriteFile(filePath, imgData, 0o644); err != nil {
			log.Printf("[image] failed to write image %d to disk: %v", i, err)
			failedImages++
			continue
		}

		// Create attachment record
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
			log.Printf("[image] failed to create attachment record: %v", err)
			failedImages++
			continue
		}

		attachments = append(attachments, *attachment)
		imageMarkdownParts = append(imageMarkdownParts,
			fmt.Sprintf("![Generated image](/v1/attachments/%s/download)", attachment.ID),
		)
	}

	if len(attachments) == 0 {
		// All images failed — clean up user message and return error
		_ = h.msgRepo.Delete(convoID, userMsg.ID)
		respondError(w, http.StatusInternalServerError, "failed to save any generated images")
		return
	}

	// Build assistant message content
	content := ""
	if len(imgResp.Images) > 0 && imgResp.Images[0].RevisedPrompt != "" {
		content = fmt.Sprintf("*Revised prompt: %s*\n\n", imgResp.Images[0].RevisedPrompt)
	}
	for _, md := range imageMarkdownParts {
		content += md + "\n\n"
	}
	if failedImages > 0 {
		content += fmt.Sprintf("*⚠️ %d of %d images failed to save.*\n\n", failedImages, len(imgResp.Images))
	}

	providerStr := imgResp.Provider
	modelStr := imgResp.Model

	assistantMsg := &models.Message{
		ID:             uuid.New().String(),
		ConversationID: convoID,
		Role:           "assistant",
		Content:        content,
		CreatedAt:      time.Now().UTC(),
		Provider:       &providerStr,
		Model:          &modelStr,
		LatencyMs:      &latency,
		MetadataJSON:   `{"type":"image_generation"}`,
	}

	if _, err := h.msgRepo.Create(assistantMsg); err != nil {
		respondInternalError(w, err)
		return
	}

	// Link attachments to the assistant message
	for _, a := range attachments {
		if err := h.attachRepo.LinkToMessage(a.ID, assistantMsg.ID); err != nil {
			log.Printf("[image] failed to link attachment %s to message: %v", a.ID, err)
		}
	}

	respondJSON(w, http.StatusCreated, imageGenerateResponse{
		UserMessage:      *userMsg,
		AssistantMessage: *assistantMsg,
		Attachments:      attachments,
	})
}
