package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/jobs"
)

// JobStatusTool retrieves, lists, or briefly awaits durable asynchronous jobs.
type JobStatusTool struct{ manager *jobs.Manager }

func NewJobStatusTool(manager *jobs.Manager) *JobStatusTool { return &JobStatusTool{manager: manager} }

func (t *JobStatusTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "job_status", Description: "Get, list, or briefly await asynchronous image, music, video, rendering, research, or export jobs.",
		Category: "jobs", Enabled: t.manager != nil, Version: "1", Risk: RiskLow, ReadOnly: true,
		SupportsParallel: true, DefaultTimeoutMS: 28000, MaxResultBytes: 131072,
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"operation":{"type":"string","enum":["get","list","await"],"default":"get"},
				"job_id":{"type":"string"},
				"wait_seconds":{"type":"integer","minimum":1,"maximum":25,"default":10},
				"limit":{"type":"integer","minimum":1,"maximum":100,"default":20}
			}
		}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
}

type jobStatusArgs struct {
	Operation   string `json:"operation"`
	JobID       string `json:"job_id"`
	WaitSeconds int    `json:"wait_seconds"`
	Limit       int    `json:"limit"`
}

func (t *JobStatusTool) Validate(raw json.RawMessage) error {
	if t.manager == nil {
		return fmt.Errorf("job manager unavailable")
	}
	var args jobStatusArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	op := strings.ToLower(strings.TrimSpace(args.Operation))
	if op == "" {
		op = "get"
	}
	if op != "get" && op != "list" && op != "await" {
		return fmt.Errorf("operation must be get, list, or await")
	}
	if op != "list" && strings.TrimSpace(args.JobID) == "" {
		return fmt.Errorf("job_id is required")
	}
	if args.WaitSeconds < 0 || args.WaitSeconds > 25 {
		return fmt.Errorf("wait_seconds must be between 1 and 25")
	}
	return nil
}

func (t *JobStatusTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args jobStatusArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	op := strings.ToLower(strings.TrimSpace(args.Operation))
	if op == "" {
		op = "get"
	}
	scope := InvocationScopeFromContext(ctx)
	if op == "list" {
		items, err := t.manager.List(jobs.Scope{UserID: scope.UserID, WorkspaceID: scope.WorkspaceID, ConversationID: scope.ConversationID}, args.Limit)
		if err != nil {
			return nil, err
		}
		encoded, _ := json.Marshal(items)
		return &ToolResult{Content: summarizeJobs(items), Structured: encoded, Metadata: map[string]interface{}{"count": len(items)}}, nil
	}

	wait := args.WaitSeconds
	if wait <= 0 {
		wait = 10
	}
	deadline := time.Now().Add(time.Duration(wait) * time.Second)
	for {
		job, err := t.manager.Get(args.JobID)
		if err != nil {
			return nil, err
		}
		if job == nil || (scope.UserID != "" && job.UserID != "" && scope.UserID != job.UserID) {
			return nil, fmt.Errorf("job not found")
		}
		if op != "await" || isTerminalJob(job.Status) || time.Now().After(deadline) {
			encoded, _ := json.Marshal(job)
			return &ToolResult{Content: summarizeJob(job), Structured: encoded, Metadata: map[string]interface{}{"job_id": job.ID, "status": job.Status, "progress": job.Progress}}, nil
		}
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// JobCancelTool cancels an owned asynchronous job and always requires approval.
type JobCancelTool struct{ manager *jobs.Manager }

func NewJobCancelTool(manager *jobs.Manager) *JobCancelTool { return &JobCancelTool{manager: manager} }

func (t *JobCancelTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "job_cancel", Description: "Cancel a queued or running asynchronous job.",
		Category: "jobs", Enabled: t.manager != nil, Version: "1", Risk: RiskHigh,
		SideEffecting: true, DefaultTimeoutMS: 5000, MaxResultBytes: 8192,
		Parameters: json.RawMessage(`{"type":"object","required":["job_id"],"properties":{"job_id":{"type":"string"}}}`),
	}
}

func (t *JobCancelTool) Validate(raw json.RawMessage) error {
	var args struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.JobID) == "" {
		return fmt.Errorf("job_id is required")
	}
	return nil
}

func (t *JobCancelTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if err := t.manager.Cancel(args.JobID, InvocationScopeFromContext(ctx).UserID); err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(map[string]interface{}{"job_id": args.JobID, "cancelled": true})
	return &ToolResult{Content: "Job cancelled: " + args.JobID, Structured: encoded}, nil
}

func isTerminalJob(status string) bool {
	return status == jobs.StatusCompleted || status == jobs.StatusFailed || status == jobs.StatusCancelled
}

func summarizeJob(job *jobs.Job) string {
	if job == nil {
		return "Job not found."
	}
	message := fmt.Sprintf("Job %s (%s): %s, %.0f%%", job.ID, job.Kind, job.Status, job.Progress*100)
	if job.Stage != "" {
		message += " — " + job.Stage
	}
	if job.ErrorMessage != "" {
		message += "\nError: " + job.ErrorMessage
	}
	if job.Status == jobs.StatusCompleted && len(job.Result) > 0 && string(job.Result) != "{}" {
		message += "\nResult: " + string(job.Result)
	}
	return message
}

func summarizeJobs(items []jobs.Job) string {
	if len(items) == 0 {
		return "No matching jobs."
	}
	var builder strings.Builder
	for _, job := range items {
		builder.WriteString(fmt.Sprintf("- %s (%s): %s, %.0f%%\n", job.ID, job.Kind, job.Status, job.Progress*100))
	}
	return strings.TrimSpace(builder.String())
}
