package api

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/go-chi/chi/v5"
)

// TitleHandler generates conversation titles using the LLM.
type TitleHandler struct {
	convoRepo *repository.ConversationRepo
	msgRepo   *repository.MessageRepo
	llmSvc    *llm.Service
}

// NewTitleHandler creates a new TitleHandler.
func NewTitleHandler(convoRepo *repository.ConversationRepo, msgRepo *repository.MessageRepo, llmSvc *llm.Service) *TitleHandler {
	return &TitleHandler{
		convoRepo: convoRepo,
		msgRepo:   msgRepo,
		llmSvc:    llmSvc,
	}
}

const titlePrompt = `Generate a very short, concise title (3-6 words max) for a conversation that starts with this message. 
Rules:
- No quotes, no punctuation at the end
- No "User asks about..." prefix 
- Be specific and descriptive
- Capitalize like a title

Message: %s

Title:`

// Generate creates an auto-title for a conversation based on its first message.
func (h *TitleHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if !verifyConversationAccess(w, r, h.convoRepo) {
		return
	}
	convoID := chi.URLParam(r, "conversationId")

	// Get the first user message
	messages, err := h.msgRepo.ListByConversation(convoID)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	var firstUserMsg string
	for _, m := range messages {
		if m.Role == "user" {
			firstUserMsg = m.Content
			break
		}
	}

	if firstUserMsg == "" {
		respondJSON(w, http.StatusOK, map[string]string{"title": "New Conversation"})
		return
	}

	// Truncate very long messages
	if len(firstUserMsg) > 500 {
		firstUserMsg = firstUserMsg[:500]
	}

	prompt := strings.Replace(titlePrompt, "%s", firstUserMsg, 1)

	llmReq := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "user", Content: prompt},
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), llm.TitleTimeout)
	defer cancel()

	resp, err := h.llmSvc.ChatComplete(ctx, llmReq)
	if err != nil {
		log.Printf("[title] LLM error: %v, falling back to truncation", err)
		// Fallback: use first 40 chars of the message
		title := firstUserMsg
		if len(title) > 40 {
			title = title[:40] + "..."
		}
		respondJSON(w, http.StatusOK, map[string]string{"title": title})
		return
	}

	title := strings.TrimSpace(resp.Content)
	// Remove quotes if the LLM wrapped it
	title = strings.Trim(title, `"'`)
	if title == "" {
		title = "New Conversation"
	}
	// Cap at 60 chars
	if len(title) > 60 {
		title = title[:60]
	}

	// Save the title
	newTitle := title
	_, err = h.convoRepo.Update(convoID, "", repository.ConversationUpdate{Title: &newTitle})
	if err != nil {
		log.Printf("[title] failed to save title: %v", err)
	}

	respondJSON(w, http.StatusOK, map[string]string{"title": title})
}
