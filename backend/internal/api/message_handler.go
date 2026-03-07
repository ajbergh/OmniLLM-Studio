package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/rag"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/websearch"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MessageHandler handles message API endpoints.
type MessageHandler struct {
	msgRepo       *repository.MessageRepo
	convoRepo     *repository.ConversationRepo
	attachRepo    *repository.AttachmentRepo
	storageDir    string
	llmSvc        *llm.Service
	orchestrator  *websearch.Orchestrator
	retriever     *rag.Retriever
	settingsRepo  *repository.SettingsRepo
	chunkRepo     *repository.ChunkRepo
	embeddingRepo *repository.EmbeddingRepo
}

// NewMessageHandler creates a new MessageHandler.
func NewMessageHandler(
	msgRepo *repository.MessageRepo,
	convoRepo *repository.ConversationRepo,
	attachRepo *repository.AttachmentRepo,
	storageDir string,
	llmSvc *llm.Service,
	orch *websearch.Orchestrator,
	retriever *rag.Retriever,
	settingsRepo *repository.SettingsRepo,
	chunkRepo *repository.ChunkRepo,
	embeddingRepo *repository.EmbeddingRepo,
) *MessageHandler {
	return &MessageHandler{
		msgRepo:       msgRepo,
		convoRepo:     convoRepo,
		attachRepo:    attachRepo,
		storageDir:    storageDir,
		llmSvc:        llmSvc,
		orchestrator:  orch,
		retriever:     retriever,
		settingsRepo:  settingsRepo,
		chunkRepo:     chunkRepo,
		embeddingRepo: embeddingRepo,
	}
}

func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	convoID := chi.URLParam(r, "conversationId")
	msgs, err := h.msgRepo.ListByConversation(convoID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if msgs == nil {
		respondJSON(w, http.StatusOK, []struct{}{})
		return
	}
	respondJSON(w, http.StatusOK, msgs)
}

// DeleteMessage removes a single message, scoped to the conversation.
func (h *MessageHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	convoID := chi.URLParam(r, "conversationId")
	msgID := chi.URLParam(r, "messageId")
	if err := h.msgRepo.Delete(convoID, msgID); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// EditMessage updates a user message's content.
func (h *MessageHandler) EditMessage(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	msgID := chi.URLParam(r, "messageId")
	convoID := chi.URLParam(r, "conversationId")

	var req struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Content == "" {
		respondError(w, http.StatusBadRequest, "content is required")
		return
	}

	// Delete the original message and everything after it (for re-generation)
	if err := h.msgRepo.DeleteFromMessageOnward(convoID, msgID); err != nil {
		if err.Error() == "message not found or does not belong to conversation" {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondInternalError(w, err)
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted_onward"})
}

type sendMessageRequest struct {
	Content       string   `json:"content"`
	AttachmentIDs []string `json:"attachment_ids,omitempty"`
	Override      *struct {
		Provider     *string `json:"provider,omitempty"`
		Model        *string `json:"model,omitempty"`
		SystemPrompt *string `json:"system_prompt,omitempty"`
	} `json:"override,omitempty"`
	WebSearch *bool `json:"web_search,omitempty"`
	Think     *bool `json:"think,omitempty"` // Ollama-only: enable/disable thinking
}

// Create handles non-streaming message creation + LLM response.
func (h *MessageHandler) Create(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	userID := auth.UserIDFromContext(r.Context())

	var req sendMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		respondError(w, http.StatusBadRequest, "content is required")
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

	// Save user message
	userMsg := &models.Message{
		ID:             uuid.New().String(),
		ConversationID: convoID,
		Role:           "user",
		Content:        req.Content,
		CreatedAt:      time.Now().UTC(),
	}
	if _, err := h.msgRepo.Create(userMsg); err != nil {
		respondInternalError(w, err)
		return
	}

	// Build LLM request
	history, err := h.msgRepo.ListByConversation(convoID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	llmReq := h.buildLLMRequest(convo, history, req.Override)
	llmReq.Think = req.Think

	// Inject attachment context into the last user message if applicable
	if attachCtx := h.linkAndBuildAttachmentContext(req.AttachmentIDs, userMsg.ID, convoID); attachCtx != "" {
		// Append attachment context to the last message (which is the user message)
		if len(llmReq.Messages) > 0 {
			last := &llmReq.Messages[len(llmReq.Messages)-1]
			last.Content = last.Content + "\n\n" + attachCtx
		}
	}

	// Auto-index attachments for RAG in the background
	if len(req.AttachmentIDs) > 0 {
		aids := req.AttachmentIDs
		cid := convoID
		go h.autoIndexForRAG(aids, cid)
	}

	// ----- RAG context injection -----
	var ragSources []rag.SourceRef
	ragCtx := h.injectRAGContext(r.Context(), convoID, req.Content, &llmReq)
	if ragCtx != nil {
		ragSources = ragCtx.Sources
	}

	// ----- Web search orchestration -----
	providerName := llmReq.Provider
	modelName := llmReq.Model

	webSearchEnabled := req.WebSearch == nil || *req.WebSearch

	var orchResult *websearch.OrchestratorResult
	if webSearchEnabled {
		orchResult, err = h.orchestrator.Process(r.Context(), req.Content, llmReq.Messages, providerName, modelName)
		if err != nil {
			log.Printf("ERROR: orchestrator: %v", err)
			respondError(w, http.StatusBadGateway, "orchestrator error")
			return
		}
	}

	if orchResult != nil {
		// If web search was attempted but failed, fall through to normal LLM path
		if orchResult.SearchFailed {
			// Prepend a note to the system context about failed search
			llmReq.Messages = append([]llm.ChatMessage{{
				Role:    "system",
				Content: "Note: A web search was attempted for this query but returned no results. Answer from your training data and mention that the information may not be current.",
			}}, llmReq.Messages...)
			// Fall through to normal LLM path below
		} else {
			// Web search was triggered – use orchestrator result
			metadata := map[string]interface{}{
				"web_search": true,
				"tool":       "web_search",
				"sources":    orchResult.Sources,
			}
			if orchResult.ToolCall != nil {
				metadata["tool_call"] = orchResult.ToolCall
			}
			metaJSON, _ := json.Marshal(metadata)

			respProvider := orchResult.Provider
			respModel := orchResult.Model

			assistantMsg := &models.Message{
				ID:             uuid.New().String(),
				ConversationID: convoID,
				Role:           "assistant",
				Content:        orchResult.Content,
				CreatedAt:      time.Now().UTC(),
				Provider:       &respProvider,
				Model:          &respModel,
				TokenInput:     orchResult.TokenInput,
				TokenOutput:    orchResult.TokenOut,
				MetadataJSON:   string(metaJSON),
			}
			if _, err := h.msgRepo.Create(assistantMsg); err != nil {
				respondInternalError(w, err)
				return
			}
			_ = h.convoRepo.TouchUpdatedAt(convoID)
			respondJSON(w, http.StatusOK, assistantMsg)
			return
		}
	}

	// ----- Normal LLM path (no web search) -----

	// Call LLM (non-streaming)
	start := time.Now()
	resp, err := h.llmSvc.ChatComplete(r.Context(), llmReq)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		log.Printf("ERROR: LLM chat complete: %v", err)
		respondError(w, http.StatusBadGateway, "LLM request failed")
		return
	}

	// Save assistant message
	assistantMsg := &models.Message{
		ID:             uuid.New().String(),
		ConversationID: convoID,
		Role:           "assistant",
		Content:        resp.Content,
		CreatedAt:      time.Now().UTC(),
		Provider:       &resp.Provider,
		Model:          &resp.Model,
		TokenInput:     resp.TokenInput,
		TokenOutput:    resp.TokenOutput,
		LatencyMs:      &latency,
	}

	// Attach RAG metadata if sources were used
	if len(ragSources) > 0 {
		metaMap := map[string]interface{}{
			"rag_sources": ragSources,
		}
		metaBytes, _ := json.Marshal(metaMap)
		assistantMsg.MetadataJSON = string(metaBytes)
	}

	if _, err := h.msgRepo.Create(assistantMsg); err != nil {
		respondInternalError(w, err)
		return
	}

	_ = h.convoRepo.TouchUpdatedAt(convoID)

	respondJSON(w, http.StatusOK, assistantMsg)
}

// Stream handles SSE streaming message creation.
func (h *MessageHandler) Stream(w http.ResponseWriter, r *http.Request) {
	convoID := chi.URLParam(r, "conversationId")
	userID := auth.UserIDFromContext(r.Context())

	var req sendMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		respondError(w, http.StatusBadRequest, "content is required")
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

	// Save user message
	userMsg := &models.Message{
		ID:             uuid.New().String(),
		ConversationID: convoID,
		Role:           "user",
		Content:        req.Content,
		CreatedAt:      time.Now().UTC(),
	}
	if _, err := h.msgRepo.Create(userMsg); err != nil {
		respondInternalError(w, err)
		return
	}

	// Build LLM request
	history, err := h.msgRepo.ListByConversation(convoID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	llmReq := h.buildLLMRequest(convo, history, req.Override)
	llmReq.Think = req.Think

	// Inject attachment context into the last user message if applicable
	if attachCtx := h.linkAndBuildAttachmentContext(req.AttachmentIDs, userMsg.ID, convoID); attachCtx != "" {
		if len(llmReq.Messages) > 0 {
			last := &llmReq.Messages[len(llmReq.Messages)-1]
			last.Content = last.Content + "\n\n" + attachCtx
		}
	}

	// Auto-index attachments for RAG in the background
	if len(req.AttachmentIDs) > 0 {
		aids := req.AttachmentIDs
		cid := convoID
		go h.autoIndexForRAG(aids, cid)
	}

	// ----- RAG context injection (streaming) -----
	var ragSources []rag.SourceRef
	ragCtx := h.injectRAGContext(r.Context(), convoID, req.Content, &llmReq)
	if ragCtx != nil {
		ragSources = ragCtx.Sources
	}

	// Set SSE headers
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Disable write deadline for this SSE connection (server has WriteTimeout set).
	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	msgID := uuid.New().String()
	var fullContent string
	var fullThinking string
	var provider, model string

	start := time.Now()

	// Send initial event with message ID
	sendSSE(w, flusher, "start", map[string]string{
		"message_id":      msgID,
		"user_message_id": userMsg.ID,
	})

	// ----- Web search orchestration for streaming -----
	providerName := llmReq.Provider
	modelName := llmReq.Model

	// Web search: skip entirely when the client explicitly disables it.
	webSearchEnabled := req.WebSearch == nil || *req.WebSearch // default true

	var searchResp *websearch.SearchResponse
	var wsLLMReq *llm.ChatRequest
	var toolCall *websearch.ToolCall
	var wsErr error
	if webSearchEnabled {
		searchResp, wsLLMReq, toolCall, wsErr = h.orchestrator.ProcessStream(
			r.Context(), req.Content, providerName, modelName,
		)
	}

	if searchResp != nil && wsErr == nil {
		// Web search was triggered – notify the UI, then stream summarizer
		sendSSE(w, flusher, "web_search", map[string]interface{}{
			"tool_call": toolCall,
			"status":    "searching",
		})

		// Send search results metadata
		sourcesJSON, _ := json.Marshal(searchResp.Results)
		sendSSE(w, flusher, "web_search_results", map[string]interface{}{
			"query":   searchResp.Query,
			"results": json.RawMessage(sourcesJSON),
		})

		// Stream the summarizer LLM response
		err = h.llmSvc.ChatStream(r.Context(), *wsLLMReq, func(chunk llm.StreamChunk) {
			fullContent += chunk.Content
			provider = chunk.Provider
			model = chunk.Model

			if chunk.Thinking != "" {
				fullThinking += chunk.Thinking
				sendSSE(w, flusher, "thinking", map[string]string{
					"content": chunk.Thinking,
				})
			}
			if chunk.Content != "" {
				sendSSE(w, flusher, "token", map[string]string{
					"content": chunk.Content,
				})
			}
		})
	} else {
		// Normal LLM streaming (no web search, or search failed — fall through)
		if wsErr != nil && toolCall != nil {
			// Web search was attempted but failed — notify UI and add context
			sendSSE(w, flusher, "web_search", map[string]interface{}{
				"tool_call": toolCall,
				"status":    "failed",
			})
			llmReq.Messages = append([]llm.ChatMessage{{
				Role:    "system",
				Content: "Note: A web search was attempted for this query but returned no results. Answer from your training data and mention that the information may not be current.",
			}}, llmReq.Messages...)
		}

		err = h.llmSvc.ChatStream(r.Context(), llmReq, func(chunk llm.StreamChunk) {
			fullContent += chunk.Content
			provider = chunk.Provider
			model = chunk.Model

			if chunk.Thinking != "" {
				fullThinking += chunk.Thinking
				sendSSE(w, flusher, "thinking", map[string]string{
					"content": chunk.Thinking,
				})
			}
			if chunk.Content != "" {
				sendSSE(w, flusher, "token", map[string]string{
					"content": chunk.Content,
				})
			}
		})
	}

	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		log.Printf("ERROR: LLM stream: %v", err)
		sendSSE(w, flusher, "error", map[string]string{"error": "internal server error"})
		return
	}

	// Build metadata for the saved message
	metaMap := map[string]interface{}{}
	if searchResp != nil {
		metaMap["web_search"] = true
		metaMap["tool"] = "web_search"
		metaMap["sources"] = searchResp.Results
		if toolCall != nil {
			metaMap["tool_call"] = toolCall
		}
	}
	if len(ragSources) > 0 {
		metaMap["rag_sources"] = ragSources
	}
	if fullThinking != "" {
		metaMap["thinking"] = fullThinking
	}

	var metaJSON string
	if len(metaMap) > 0 {
		metaBytes, _ := json.Marshal(metaMap)
		metaJSON = string(metaBytes)
	}

	// Save assistant message
	pProvider := &provider
	pModel := &model
	assistantMsg := &models.Message{
		ID:             msgID,
		ConversationID: convoID,
		Role:           "assistant",
		Content:        fullContent,
		CreatedAt:      time.Now().UTC(),
		Provider:       pProvider,
		Model:          pModel,
		LatencyMs:      &latency,
		MetadataJSON:   metaJSON,
	}
	if _, err := h.msgRepo.Create(assistantMsg); err != nil {
		log.Printf("error saving streamed message: %v", err)
	}

	_ = h.convoRepo.TouchUpdatedAt(convoID)

	// Send done event
	donePayload := map[string]interface{}{
		"message_id": msgID,
		"provider":   provider,
		"model":      model,
		"latency_ms": latency,
	}
	if searchResp != nil {
		donePayload["web_search"] = true
		donePayload["sources"] = searchResp.Results
	}
	if len(ragSources) > 0 {
		donePayload["rag_sources"] = ragSources
	}
	if fullThinking != "" {
		donePayload["thinking"] = fullThinking
	}
	sendSSE(w, flusher, "done", donePayload)
}

func sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonData))
	flusher.Flush()
}

// injectRAGContext checks if RAG is enabled, retrieves relevant chunks, and
// prepends a RAG system message into the LLM request. Returns the context
// block (with sources) or nil if RAG is disabled / no results.
func (h *MessageHandler) injectRAGContext(
	ctx context.Context,
	conversationID string,
	query string,
	llmReq *llm.ChatRequest,
) *rag.ContextBlock {
	if h.retriever == nil || h.settingsRepo == nil {
		return nil
	}

	settings, err := h.settingsRepo.GetTyped()
	if err != nil || !settings.RAGEnabled {
		return nil
	}

	topK := settings.RAGTopK
	if topK <= 0 {
		topK = 5
	}

	chunks, err := h.retriever.Retrieve(
		ctx,
		conversationID,
		query,
		llmReq.Provider,
		settings.RAGEmbeddingModel,
		topK,
	)
	if err != nil {
		log.Printf("[rag] retrieval error for conversation %s: %v", conversationID, err)
		return nil
	}
	if len(chunks) == 0 {
		return nil
	}

	block := rag.BuildContext(chunks)
	if block == nil {
		return nil
	}

	// Prepend the RAG system message into the LLM request
	ragMsg := llm.ChatMessage{
		Role:    "system",
		Content: rag.SystemPrompt(block),
	}
	llmReq.Messages = append([]llm.ChatMessage{ragMsg}, llmReq.Messages...)

	return block
}

func (h *MessageHandler) buildLLMRequest(
	convo *models.Conversation,
	history []models.Message,
	override *struct {
		Provider     *string `json:"provider,omitempty"`
		Model        *string `json:"model,omitempty"`
		SystemPrompt *string `json:"system_prompt,omitempty"`
	},
) llm.ChatRequest {
	req := llm.ChatRequest{}

	// Determine provider/model (override > conversation default > global default)
	if override != nil && override.Provider != nil {
		req.Provider = *override.Provider
	} else if convo.DefaultProvider != nil {
		req.Provider = *convo.DefaultProvider
	}

	if override != nil && override.Model != nil {
		req.Model = *override.Model
	} else if convo.DefaultModel != nil {
		req.Model = *convo.DefaultModel
	}

	// System prompt
	var systemPrompt string
	if override != nil && override.SystemPrompt != nil {
		systemPrompt = *override.SystemPrompt
	} else if convo.SystemPrompt != nil {
		systemPrompt = *convo.SystemPrompt
	}

	if systemPrompt != "" {
		req.Messages = append(req.Messages, llm.ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// Add history
	for _, m := range history {
		req.Messages = append(req.Messages, llm.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	return req
}

// linkAndBuildAttachmentContext links attachments to a message and returns a
// context string describing their contents (for inclusion in the LLM prompt).
// For text-based files (including PDFs with extractable text) it includes
// content; for images/binary it includes only a description with metadata.
func (h *MessageHandler) linkAndBuildAttachmentContext(attachmentIDs []string, messageID, conversationID string) string {
	if len(attachmentIDs) == 0 || h.attachRepo == nil {
		return ""
	}

	var parts []string
	for _, aid := range attachmentIDs {
		a, err := h.attachRepo.GetByID(aid)
		if err != nil || a == nil {
			continue
		}

		// Reject attachments that belong to a different conversation
		if a.ConversationID != conversationID {
			log.Printf("[attach] attachment %s belongs to conversation %s, not %s — skipping", aid, a.ConversationID, conversationID)
			continue
		}

		// Link to the user message
		if err := h.attachRepo.LinkToMessage(aid, messageID); err != nil {
			log.Printf("[attach] failed to link %s to message %s: %v", aid, messageID, err)
		}

		// Build context based on MIME type
		if canExtractAttachmentText(a.MimeType) {
			safePath, pathErr := SafeJoin(h.storageDir, a.StoragePath)
			if pathErr != nil {
				parts = append(parts, fmt.Sprintf("[Attached file: %s (invalid path)]", a.MimeType))
				continue
			}
			content, err := extractAttachmentText(safePath, a.MimeType)
			if err != nil {
				parts = append(parts, fmt.Sprintf("[Attached file: %s (text extraction error)]", a.MimeType))
				continue
			}
			// Truncate very large files
			const maxLen = 8000
			if len(content) > maxLen {
				content = content[:maxLen] + "\n... (truncated)"
			}
			parts = append(parts, fmt.Sprintf("[Attached file (%s, %d bytes)]:\n%s", a.MimeType, a.Bytes, content))
		} else {
			parts = append(parts, fmt.Sprintf("[Attached file: %s, %d bytes — binary content not included]", a.MimeType, a.Bytes))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

// autoIndexForRAG asynchronously indexes text-extractable attachments for RAG
// retrieval. Runs in a background goroutine so it doesn't block the response.
func (h *MessageHandler) autoIndexForRAG(attachmentIDs []string, conversationID string) {
	if h.chunkRepo == nil || h.embeddingRepo == nil || h.settingsRepo == nil || h.llmSvc == nil {
		return
	}
	settings, err := h.settingsRepo.GetTyped()
	if err != nil || !settings.RAGEnabled {
		return
	}

	// Resolve embedding provider from conversation
	embeddingProvider := ""
	if convo, err := h.convoRepo.GetByID(conversationID); err == nil && convo != nil && convo.DefaultProvider != nil {
		embeddingProvider = *convo.DefaultProvider
	}

	chunkOpts := rag.ChunkOptions{
		ChunkSize: settings.RAGChunkSize,
		Overlap:   settings.RAGChunkOverlap,
	}
	if chunkOpts.ChunkSize <= 0 {
		chunkOpts = rag.DefaultChunkOptions()
	}

	for _, aid := range attachmentIDs {
		a, err := h.attachRepo.GetByID(aid)
		if err != nil || a == nil || !canExtractAttachmentText(a.MimeType) {
			continue
		}

		// Skip if already indexed
		existing, _ := h.chunkRepo.ListByAttachment(aid)
		if len(existing) > 0 {
			continue
		}

		safePath, pathErr := SafeJoin(h.storageDir, a.StoragePath)
		if pathErr != nil {
			log.Printf("[rag-auto] unsafe path for attachment %s: %v", aid, pathErr)
			continue
		}
		content, err := extractAttachmentText(safePath, a.MimeType)
		if err != nil {
			log.Printf("[rag-auto] text extraction failed for %s: %v", aid, err)
			continue
		}

		dbChunks := rag.DetectAndChunk(content, a.MimeType, a.ID, conversationID, chunkOpts)
		if len(dbChunks) == 0 {
			continue
		}

		if err := h.chunkRepo.CreateBatch(dbChunks); err != nil {
			log.Printf("[rag-auto] chunk creation failed for %s: %v", aid, err)
			continue
		}

		texts := make([]string, len(dbChunks))
		for i, c := range dbChunks {
			texts[i] = c.Content
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		embResp, err := h.llmSvc.Embed(ctx, llm.EmbeddingRequest{
			Provider: embeddingProvider,
			Model:    settings.RAGEmbeddingModel,
			Input:    texts,
		})
		cancel()
		if err != nil {
			log.Printf("[rag-auto] embedding failed for %s: %v", aid, err)
			continue
		}

		embModels := make([]models.DocumentEmbedding, len(embResp.Embeddings))
		for i, vec := range embResp.Embeddings {
			embModels[i] = models.DocumentEmbedding{
				ChunkID:    dbChunks[i].ID,
				Embedding:  vec,
				Model:      embResp.Model,
				Dimensions: embResp.Dimensions,
			}
		}

		if err := h.embeddingRepo.UpsertBatch(embModels); err != nil {
			log.Printf("[rag-auto] embedding storage failed for %s: %v", aid, err)
			continue
		}

		log.Printf("[rag-auto] indexed attachment %s: %d chunks, %d embeddings", aid, len(dbChunks), len(embModels))
	}
}

// isTextMIME returns true for MIME types whose content can be included as text context.
func isTextMIME(mime string) bool {
	if strings.HasPrefix(mime, "text/") {
		return true
	}
	textTypes := []string{
		"application/json",
		"application/xml",
		"application/javascript",
		"application/typescript",
		"application/x-yaml",
		"application/yaml",
		"application/toml",
		"application/x-sh",
		"application/sql",
		"application/csv",
	}
	for _, t := range textTypes {
		if mime == t {
			return true
		}
	}
	return false
}
