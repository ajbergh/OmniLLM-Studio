package sandbox

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RestoreSession rehydrates one trusted Broker mapping for a runtime session
// that was durably recorded before a control-plane restart. It never creates a
// runtime and is intentionally package-level control-plane infrastructure, not a
// model-facing API. The caller must supply the original owner and immutable
// session metadata.
func (b *Broker) RestoreSession(_ context.Context, session Session) error {
	if b == nil || b.runtime == nil {
		return fmt.Errorf("sandbox broker is unavailable")
	}
	if session.Owner.Empty() || strings.TrimSpace(session.ID) == "" || strings.TrimSpace(session.RuntimeID) == "" {
		return fmt.Errorf("restored sandbox session requires owner, session id, and runtime id")
	}
	if !strings.HasPrefix(session.ID, "sbx_") {
		return fmt.Errorf("invalid restored sandbox session id")
	}
	if session.CreatedAt.IsZero() || session.ExpiresAt.IsZero() || !session.ExpiresAt.After(session.CreatedAt) {
		return fmt.Errorf("invalid restored sandbox session lifetime")
	}
	if time.Until(session.ExpiresAt) <= 0 {
		return fmt.Errorf("cannot restore expired sandbox session")
	}
	if err := validateCreateRequest(session.Spec); err != nil {
		return fmt.Errorf("invalid restored sandbox specification: %w", err)
	}
	if err := requireCapabilities(b.runtime.Capabilities(), session.Spec.Requirements); err != nil {
		return err
	}
	if err := requireEnforceableResourceLimits(b.runtime.Capabilities(), session.Spec.Resources); err != nil {
		return err
	}

	restored := cloneSession(session)
	b.mu.Lock()
	defer b.mu.Unlock()
	if existing, ok := b.sessions[session.ID]; ok {
		if !existing.Owner.Equal(session.Owner) || existing.RuntimeID != session.RuntimeID {
			return fmt.Errorf("sandbox session restoration conflicts with existing mapping")
		}
		return nil
	}
	b.sessions[session.ID] = restored
	return nil
}

// DestroyRecordedRuntime is the recovery-only cleanup path for a runtime whose
// Broker mapping was lost during a control-plane restart. The durable task layer
// supplies the exact owner/session/runtime association captured before Exec.
// This method deliberately does not authorize by the in-memory session map: its
// purpose is to destroy, never to execute in or widen authority to, a recovered
// runtime.
func (b *Broker) DestroyRecordedRuntime(ctx context.Context, owner OwnerScope, sessionID, runtimeID string) error {
	if b == nil || b.runtime == nil {
		return fmt.Errorf("sandbox broker is unavailable")
	}
	if owner.Empty() {
		return fmt.Errorf("sandbox owner scope is required")
	}
	sessionID = strings.TrimSpace(sessionID)
	runtimeID = strings.TrimSpace(runtimeID)
	if sessionID == "" || runtimeID == "" || !strings.HasPrefix(sessionID, "sbx_") {
		return fmt.Errorf("recorded sandbox session and runtime ids are required")
	}
	if err := b.runtime.Destroy(ctx, runtimeID); err != nil {
		return err
	}
	b.mu.Lock()
	if existing, ok := b.sessions[sessionID]; ok && existing.Owner.Equal(owner) && existing.RuntimeID == runtimeID {
		delete(b.sessions, sessionID)
	}
	b.mu.Unlock()
	return nil
}
