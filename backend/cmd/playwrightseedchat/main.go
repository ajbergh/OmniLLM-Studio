package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/google/uuid"
)

type seedResult struct {
	Title              string `json:"title"`
	ConversationID     string `json:"conversation_id"`
	UserMessageID      string `json:"user_message_id"`
	AssistantMessageID string `json:"assistant_message_id"`
}

const defaultAssistantContent = `Here is a table-heavy response for responsive layout testing.

| Provider | Very Long Model Identifier | Input Cost | Output Cost | Notes |
| --- | --- | ---: | ---: | --- |
| OpenAI | gpt-4.1-ultra-long-model-name-for-layout-regression | $2.00 | $8.00 | This value should scroll inside the bubble. |
| Anthropic | claude-sonnet-ultra-long-model-name-for-layout-regression | $3.00 | $15.00 | The page itself should not overflow horizontally. |
| Local | ollama-custom-experimental-model-with-very-long-name | $0.00 | $0.00 | Local provider names can be long. |

` + "```ts" + `
const longIdentifier = 'table_content_should_stay_inside_the_message_bubble_and_not_expand_the_page';
` + "```" + `
`

func main() {
	title := flag.String("title", "Responsive Table Fixture", "chat conversation title")
	userContent := flag.String("user", "Show me a provider comparison table.", "seed user message content")
	assistantContent := flag.String("assistant", defaultAssistantContent, "seed assistant message content")
	flag.Parse()

	dbPath := os.Getenv("OMNILLM_DB_PATH")
	if dbPath == "" {
		exitf("OMNILLM_DB_PATH is required")
	}

	database, err := db.Open(dbPath)
	if err != nil {
		exitErr("open db", err)
	}
	defer db.Close(database)

	convoRepo := repository.NewConversationRepo(database)
	msgRepo := repository.NewMessageRepo(database)

	convo, err := convoRepo.CreateWithKind("", *title, models.ConversationKindChat, nil, nil, nil)
	if err != nil {
		exitErr("create conversation", err)
	}

	now := time.Now().UTC()
	userMessage := &models.Message{
		ID:             uuid.New().String(),
		ConversationID: convo.ID,
		Role:           "user",
		Content:        *userContent,
		CreatedAt:      now,
		BranchID:       "main",
	}
	if _, err := msgRepo.Create(userMessage); err != nil {
		exitErr("create user message", err)
	}

	model := "layout-fixture-model"
	provider := "fixture"
	assistantMessage := &models.Message{
		ID:             uuid.New().String(),
		ConversationID: convo.ID,
		Role:           "assistant",
		Content:        *assistantContent,
		CreatedAt:      now.Add(time.Second),
		BranchID:       "main",
		Model:          &model,
		Provider:       &provider,
	}
	if _, err := msgRepo.Create(assistantMessage); err != nil {
		exitErr("create assistant message", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(seedResult{
		Title:              *title,
		ConversationID:     convo.ID,
		UserMessageID:      userMessage.ID,
		AssistantMessageID: assistantMessage.ID,
	}); err != nil {
		exitErr("encode seed result", err)
	}
}

func exitErr(action string, err error) {
	exitf("%s: %v", action, err)
}

func exitf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
