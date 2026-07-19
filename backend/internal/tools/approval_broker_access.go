package tools

import "time"

// Get returns a copy of an approval request by ID.
func (b *ApprovalBroker) Get(id string) (*PendingApproval, bool) {
	now := time.Now().UTC()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(now)
	p, ok := b.pending[id]
	if !ok {
		return nil, false
	}
	copy := *p
	copy.result = nil
	return &copy, true
}
