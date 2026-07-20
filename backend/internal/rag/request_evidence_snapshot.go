package rag

import (
	"context"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// SnapshotRequestEvidence returns a copy of request-scoped evidence without
// consuming it. Provider-native streaming uses this so a rejected native
// request can still retry through the local web-search fallback. Entries remain
// bounded by the registry's existing five-minute expiration and are consumed by
// the fallback path or cleared after a completed non-streaming native response.
func SnapshotRequestEvidence(ctx context.Context) []Evidence {
	key := middleware.GetReqID(ctx)
	if key == "" {
		return nil
	}
	requestEvidenceRegistry.Lock()
	defer requestEvidenceRegistry.Unlock()
	pruneRequestEvidenceLocked(time.Now())
	entry, ok := requestEvidenceRegistry.items[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil
	}
	return append([]Evidence(nil), entry.Evidence...)
}
