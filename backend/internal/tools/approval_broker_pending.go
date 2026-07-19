package tools

import (
	"time"

	"github.com/google/uuid"
)

// CreatePending records an approval request without blocking. This is used by
// ordinary chat, where the current response stream cannot be held open while a
// separate HTTP request resolves the decision. Durable orchestration can resume
// the pending invocation on the next run checkpoint.
func (b *ApprovalBroker) CreatePending(req ApprovalRequest) PendingApproval {
	now := time.Now().UTC()
	if req.ApprovalID == "" {
		req.ApprovalID = uuid.NewString()
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
	copy := *p
	copy.result = nil
	return copy
}
