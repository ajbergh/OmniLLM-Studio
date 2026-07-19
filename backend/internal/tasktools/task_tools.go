package tasktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/agent"
	"github.com/ajbergh/omnillm-studio/internal/tasks"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// ListTool lists scheduled tasks owned by the current user.
type ListTool struct{ scheduler *tasks.Scheduler }

func NewListTool(scheduler *tasks.Scheduler) *ListTool { return &ListTool{scheduler: scheduler} }

func (t *ListTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name: "task_list", Description: "List one-time, recurring, and condition-watch agent tasks owned by the current user.",
		Category: "automation", Enabled: t.scheduler != nil, Version: "1", Risk: tools.RiskLow,
		ReadOnly: true, SupportsParallel: true, DefaultTimeoutMS: 3000, MaxResultBytes: 65536,
		Parameters:   json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":100,"default":20}}}`),
		OutputSchema: json.RawMessage(`{"type":"array"}`),
	}
}

func (t *ListTool) Validate(raw json.RawMessage) error {
	var args struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if args.Limit < 0 || args.Limit > 100 {
		return fmt.Errorf("limit must be between 1 and 100")
	}
	return nil
}

func (t *ListTool) Execute(ctx context.Context, raw json.RawMessage) (*tools.ToolResult, error) {
	var args struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	items, err := t.scheduler.List(tools.InvocationScopeFromContext(ctx).UserID, args.Limit)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(items)
	if len(items) == 0 {
		return &tools.ToolResult{Content: "No scheduled tasks.", Structured: encoded}, nil
	}
	var builder strings.Builder
	for _, item := range items {
		builder.WriteString(fmt.Sprintf("- %s [%s] %s — next %s (task_id: %s)\n", item.Title, item.Status, item.ScheduleKind, item.NextRunAt.Format(time.RFC3339), item.ID))
	}
	return &tools.ToolResult{
		Content:    strings.TrimSpace(builder.String()),
		Structured: encoded,
		Metadata:   map[string]interface{}{"count": len(items)},
	}, nil
}

// CreateTool creates an automation and requires approval.
type CreateTool struct{ scheduler *tasks.Scheduler }

func NewCreateTool(scheduler *tasks.Scheduler) *CreateTool { return &CreateTool{scheduler: scheduler} }

func (t *CreateTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name: "task_create", Description: "Create a one-time, recurring, or condition-watch agent task. Recurring tasks must run no more frequently than hourly.",
		Category: "automation", Enabled: t.scheduler != nil, Version: "1", Risk: tools.RiskHigh,
		SideEffecting: true, DefaultTimeoutMS: 5000, MaxResultBytes: 16384,
		Parameters: json.RawMessage(`{
			"type":"object","required":["title","prompt","schedule_kind","next_run_at"],
			"properties":{
				"title":{"type":"string","maxLength":200},"prompt":{"type":"string","maxLength":20000},
				"conversation_id":{"type":"string"},
				"profile":{"type":"string","enum":["chat","research","agent"],"default":"agent"},
				"timezone":{"type":"string","default":"UTC"},
				"schedule_kind":{"type":"string","enum":["one_time","interval","condition"]},
				"next_run_at":{"type":"string","description":"RFC3339 timestamp"},
				"interval_seconds":{"type":"integer","minimum":3600}
			}
		}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
}

type createArgs struct {
	Title           string `json:"title"`
	Prompt          string `json:"prompt"`
	ConversationID  string `json:"conversation_id"`
	Profile         string `json:"profile"`
	Timezone        string `json:"timezone"`
	ScheduleKind    string `json:"schedule_kind"`
	NextRunAt       string `json:"next_run_at"`
	IntervalSeconds int64  `json:"interval_seconds"`
}

func (t *CreateTool) Validate(raw json.RawMessage) error {
	var args createArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.Title) == "" || strings.TrimSpace(args.Prompt) == "" {
		return fmt.Errorf("title and prompt are required")
	}
	if _, err := time.Parse(time.RFC3339, args.NextRunAt); err != nil {
		return fmt.Errorf("next_run_at must be RFC3339")
	}
	if args.ScheduleKind != tasks.KindOneTime && args.ScheduleKind != tasks.KindInterval && args.ScheduleKind != tasks.KindCondition {
		return fmt.Errorf("schedule_kind must be one_time, interval, or condition")
	}
	if args.ScheduleKind != tasks.KindOneTime && args.IntervalSeconds < int64(tasks.MinimumInterval/time.Second) {
		return fmt.Errorf("recurring tasks must run no more frequently than hourly")
	}
	return nil
}

func (t *CreateTool) Execute(ctx context.Context, raw json.RawMessage) (*tools.ToolResult, error) {
	var args createArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	nextRun, _ := time.Parse(time.RFC3339, args.NextRunAt)
	scope := tools.InvocationScopeFromContext(ctx)
	conversationID := args.ConversationID
	if conversationID == "" {
		conversationID = scope.ConversationID
	}
	profile := agent.RunProfile(args.Profile)
	if profile == "" {
		profile = agent.ProfileAgent
	}
	item, err := t.scheduler.Create(tasks.CreateRequest{
		UserID:          scope.UserID,
		ConversationID:  conversationID,
		Title:           args.Title,
		Prompt:          args.Prompt,
		Profile:         profile,
		Timezone:        args.Timezone,
		ScheduleKind:    args.ScheduleKind,
		NextRunAt:       nextRun,
		IntervalSeconds: args.IntervalSeconds,
	})
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(item)
	return &tools.ToolResult{
		Content:    fmt.Sprintf("Created scheduled task %s. Next run: %s.", item.ID, item.NextRunAt.Format(time.RFC3339)),
		Structured: encoded,
		Metadata:   map[string]interface{}{"task_id": item.ID},
	}, nil
}

// UpdateTool pauses, resumes, or deletes a task and requires approval.
type UpdateTool struct{ scheduler *tasks.Scheduler }

func NewUpdateTool(scheduler *tasks.Scheduler) *UpdateTool { return &UpdateTool{scheduler: scheduler} }

func (t *UpdateTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name: "task_update", Description: "Pause, resume, or delete one scheduled agent task.",
		Category: "automation", Enabled: t.scheduler != nil, Version: "1", Risk: tools.RiskHigh,
		SideEffecting: true, DefaultTimeoutMS: 5000, MaxResultBytes: 8192,
		Parameters: json.RawMessage(`{"type":"object","required":["task_id","action"],"properties":{"task_id":{"type":"string"},"action":{"type":"string","enum":["pause","resume","delete"]}}}`),
	}
}

func (t *UpdateTool) Validate(raw json.RawMessage) error {
	var args struct {
		TaskID string `json:"task_id"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.TaskID) == "" {
		return fmt.Errorf("task_id is required")
	}
	if args.Action != "pause" && args.Action != "resume" && args.Action != "delete" {
		return fmt.Errorf("action must be pause, resume, or delete")
	}
	return nil
}

func (t *UpdateTool) Execute(ctx context.Context, raw json.RawMessage) (*tools.ToolResult, error) {
	var args struct {
		TaskID string `json:"task_id"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	userID := tools.InvocationScopeFromContext(ctx).UserID
	var err error
	switch args.Action {
	case "pause":
		err = t.scheduler.SetStatus(args.TaskID, userID, tasks.StatusPaused)
	case "resume":
		err = t.scheduler.SetStatus(args.TaskID, userID, tasks.StatusActive)
	case "delete":
		err = t.scheduler.Delete(args.TaskID, userID)
	}
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(map[string]interface{}{"task_id": args.TaskID, "action": args.Action, "ok": true})
	return &tools.ToolResult{
		Content:    fmt.Sprintf("Task %s: %s complete.", args.TaskID, args.Action),
		Structured: encoded,
	}, nil
}
