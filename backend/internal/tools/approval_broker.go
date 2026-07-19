package tools

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrApprovalNotFound = errors.New("approval not found")
	ErrApprovalResolved = errors.New("approval already resolved")
)

// ApprovalDecision records a user's decision for a pending invocation.
type ApprovalDecision struct {
	Approved  bool           `json:"approved"`
	Arguments jsonRawMessage `json:"-"`
}

// jsonRawMessage is kept private so the broker does not need custom marshaling;
// callers pass updated arguments directly to Resolve.
type jsonRawMessage = []byte

// PendingApproval is safe to expose through the API. The decision channel is
// intentionally excluded from serialization.
type PendingApproval struct {
	ID         string          `json:"id"`
	Request    ApprovalRequest `json:"request"`
	Status     string          `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
	ExpiresAt  time.Time       `json:"expires_at"`
	ResolvedAt *time.Time      `json:"resolved_at,omitempty"`
	Approved   *bool           `json:"approved,omitempty"`
	result     chan approvalResolution
}

type approvalResolution struct {
	approved  bool
	arguments []byte
}

// ApprovalBroker coordinates approval requests shared by Chat and Agent mode.
// Pending requests are in-memory by design; durable invocation records preserve
// the audit trail while a disconnected run can be resumed from its checkpoint.
type ApprovalBroker struct {
	mu      sync.RWMutex
	pending map[string]*PendingApproval
	ttl     time.Duration
}

func NewApprovalBroker(ttl time.Duration) *ApprovalBroker {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &ApprovalBroker{pending: make(map[string]*PendingApproval), ttl: ttl}
}

// Request creates an approval, emits an event, and waits for a decision.
func (b *ApprovalBroker) Request(ctx context.Context, req ApprovalRequest) (bool, []byte, error) {
	now := time.Now().UTC()
	if req.ApprovalID == "" {
		req.ApprovalID = uuid.New().String()
	}
	p := &PendingApproval{
		ID:        req.ApprovalID,
		Request:   req,
		Status:    "pending",
		CreatedAt: now,
		ExpiresAt: now.Add(b.ttl),
		result:    make(chan approvalResolution, 1),
	}

	b.mu.Lock()
	b.pruneLocked(now)
	b.pending[p.ID] = p
	b.mu.Unlock()

	emitEvent(ctx, ToolEvent{
		Type:       ToolEventApprovalRequired,
		ToolCallID: req.ToolCallID,
		ToolName:   req.ToolName,
		Scope:      req.Scope,
		Data: map[string]interface{}{
			"approval_id": p.ID,
			"description": req.Description,
			"arguments":   string(req.Arguments),
			"risk":        req.Risk,
			"read_only":   req.ReadOnly,
		},
	})

	select {
	case resolution := <-p.result:
		emitEvent(ctx, ToolEvent{
			Type:       ToolEventApprovalResolved,
			ToolCallID: req.ToolCallID,
			ToolName:   req.ToolName,
			Scope:      req.Scope,
			Data: map[string]interface{}{
				"approval_id": p.ID,
				"approved":    resolution.approved,
			},
		})
		return resolution.approved, resolution.arguments, nil
	case <-ctx.Done():
		b.expire(p.ID)
		return false, nil, ctx.Err()
	}
}

// Resolve approves or rejects a pending request. Optional edited arguments are
// validated by the executor before the invocation runs.
func (b *ApprovalBroker) Resolve(id string, approved bool, arguments []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	p, ok := b.pending[id]
	if !ok {
		return ErrApprovalNotFound
	}
	if p.Status != "pending" {
		return ErrApprovalResolved
	}
	if time.Now().UTC().After(p.ExpiresAt) {
		p.Status = "expired"
		return fmt.Errorf("%w: request expired", ErrApprovalResolved)
	}

	now := time.Now().UTC()
	p.Status = "resolved"
	p.ResolvedAt = &now
	p.Approved = &approved
	p.result <- approvalResolution{approved: approved, arguments: arguments}
	close(p.result)
	return nil
}

// List returns pending requests visible to a scope. Empty fields act as
// wildcards, allowing administrators to inspect all requests.
func (b *ApprovalBroker) List(scope InvocationScope) []PendingApproval {
	now := time.Now().UTC()
	b.mu.Lock()
	b.pruneLocked(now)
	out := make([]PendingApproval, 0, len(b.pending))
	for _, p := range b.pending {
		if p.Status != "pending" || !scopeMatches(scope, p.Request.Scope) {
			continue
		}
		copy := *p
		copy.result = nil
		out = append(out, copy)
	}
	b.mu.Unlock()

	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func scopeMatches(filter, candidate InvocationScope) bool {
	if filter.UserID != "" && candidate.UserID != filter.UserID {
		return false
	}
	if filter.WorkspaceID != "" && candidate.WorkspaceID != filter.WorkspaceID {
		return false
	}
	if filter.ConversationID != "" && candidate.ConversationID != filter.ConversationID {
		return false
	}
	if filter.RunID != "" && candidate.RunID != filter.RunID {
		return false
	}
	return true
}

func (b *ApprovalBroker) expire(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if p, ok := b.pending[id]; ok && p.Status == "pending" {
		p.Status = "expired"
	}
}

func (b *ApprovalBroker) pruneLocked(now time.Time) {
	for id, p := range b.pending {
		if p.Status == "pending" && now.After(p.ExpiresAt) {
			p.Status = "expired"
		}
		if p.Status != "pending" && p.ResolvedAt != nil && now.Sub(*p.ResolvedAt) > b.ttl {
			delete(b.pending, id)
		}
	}
}
