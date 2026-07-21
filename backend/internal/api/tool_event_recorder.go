package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// ToolEventRecorder asynchronously persists normalized tool lifecycle events.
type ToolEventRecorder struct {
	db     *sql.DB
	events chan tools.ToolEvent
	cancel context.CancelFunc
	done   chan struct{}
}

func NewToolEventRecorder(db *sql.DB) *ToolEventRecorder {
	ctx, cancel := context.WithCancel(context.Background())
	recorder := &ToolEventRecorder{
		db: db, events: make(chan tools.ToolEvent, 512), cancel: cancel, done: make(chan struct{}),
	}
	go recorder.loop(ctx)
	return recorder
}

// Record is safe for streaming paths and drops only when the bounded audit
// buffer is saturated; a warning is emitted so overload is observable.
func (r *ToolEventRecorder) Record(event tools.ToolEvent) {
	select {
	case r.events <- event:
	default:
		log.Printf("WARN: tool event audit buffer full; dropping %s for %s", event.Type, event.ToolCallID)
	}
}

func (r *ToolEventRecorder) Shutdown(ctx context.Context) error {
	r.cancel()
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *ToolEventRecorder) loop(ctx context.Context) {
	defer close(r.done)
	for {
		select {
		case event := <-r.events:
			if err := r.persist(event); err != nil {
				log.Printf("WARN: persist tool event: %v", err)
			}
		case <-ctx.Done():
			for {
				select {
				case event := <-r.events:
					_ = r.persist(event)
				default:
					return
				}
			}
		}
	}
}

func (r *ToolEventRecorder) persist(event tools.ToolEvent) error {
	dataJSON, _ := json.Marshal(event.Data)
	arguments := "{}"
	if value, ok := event.Data["arguments"]; ok {
		switch typed := value.(type) {
		case string:
			if json.Valid([]byte(typed)) {
				arguments = typed
			} else {
				encoded, _ := json.Marshal(typed)
				arguments = string(encoded)
			}
		default:
			encoded, _ := json.Marshal(typed)
			arguments = string(encoded)
		}
	}
	status := string(event.Type)
	approvalStatus := ""
	if event.Type == tools.ToolEventApprovalRequired {
		approvalStatus = "required"
	}
	if event.Type == tools.ToolEventApprovalResolved {
		if approved, _ := event.Data["approved"].(bool); approved {
			approvalStatus = "approved"
		} else {
			approvalStatus = "rejected"
		}
	}
	durationMS := int64Value(event.Data["duration_ms"])
	resultBytes := int64Value(event.Data["result_bytes"])
	now := time.Now().UTC()
	startedAt, completedAt := interface{}(nil), interface{}(nil)
	if event.Type == tools.ToolEventStarted {
		startedAt = now
	}
	// Every terminal event, including request cancellation, closes the durable
	// invocation record so operational views do not leave it appearing in-flight.
	if event.Type == tools.ToolEventCompleted || event.Type == tools.ToolEventFailed || event.Type == tools.ToolEventTimedOut || event.Type == tools.ToolEventCancelled {
		completedAt = now
	}

	_, err := r.db.Exec(`
		INSERT INTO tool_invocations (
			id, tool_call_id, tool_name, user_id, workspace_id, conversation_id, message_id, run_id,
			arguments_json, status, approval_status, result_json, duration_ms, result_bytes,
			created_at, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			tool_name = excluded.tool_name,
			user_id = CASE WHEN excluded.user_id != '' THEN excluded.user_id ELSE tool_invocations.user_id END,
			workspace_id = CASE WHEN excluded.workspace_id != '' THEN excluded.workspace_id ELSE tool_invocations.workspace_id END,
			conversation_id = CASE WHEN excluded.conversation_id != '' THEN excluded.conversation_id ELSE tool_invocations.conversation_id END,
			message_id = CASE WHEN excluded.message_id != '' THEN excluded.message_id ELSE tool_invocations.message_id END,
			run_id = CASE WHEN excluded.run_id != '' THEN excluded.run_id ELSE tool_invocations.run_id END,
			arguments_json = CASE WHEN excluded.arguments_json != '{}' THEN excluded.arguments_json ELSE tool_invocations.arguments_json END,
			status = excluded.status,
			approval_status = CASE WHEN excluded.approval_status != '' THEN excluded.approval_status ELSE tool_invocations.approval_status END,
			result_json = excluded.result_json,
			duration_ms = CASE WHEN excluded.duration_ms > 0 THEN excluded.duration_ms ELSE tool_invocations.duration_ms END,
			result_bytes = CASE WHEN excluded.result_bytes > 0 THEN excluded.result_bytes ELSE tool_invocations.result_bytes END,
			started_at = COALESCE(excluded.started_at, tool_invocations.started_at),
			completed_at = COALESCE(excluded.completed_at, tool_invocations.completed_at)
	`, event.ToolCallID, event.ToolCallID, event.ToolName,
		event.Scope.UserID, event.Scope.WorkspaceID, event.Scope.ConversationID, event.Scope.MessageID, event.Scope.RunID,
		arguments, status, approvalStatus, string(dataJSON), durationMS, resultBytes,
		now, startedAt, completedAt,
	)
	return err
}

func int64Value(value interface{}) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}
