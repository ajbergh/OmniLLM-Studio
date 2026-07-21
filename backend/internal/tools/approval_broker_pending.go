package tools

import (
	"time"

	"github.com/google/uuid"
)

// CreatePending records an approval request without blocking. It remains the
// compatibility path for durable or out-of-band orchestration; ordinary Chat
// Studio calls now use the inline approval continuation path.
func (b *ApprovalBroker) CreatePending(req ApprovalRequest) PendingApproval {
	now := time.Now().UTC()
	if req.ContinuationMode == "" {
		req.ContinuationMode = "out_of_band"
	}
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
