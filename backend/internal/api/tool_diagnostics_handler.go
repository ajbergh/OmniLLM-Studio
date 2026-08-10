package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// ToolDiagnosticsHandler exposes privacy-safe operational tool telemetry for the
// authenticated user. Payload-bearing audit columns never leave the backend.
type ToolDiagnosticsHandler struct {
	invocations *repository.ToolInvocationRepo
}

func NewToolDiagnosticsHandler(invocations *repository.ToolInvocationRepo) *ToolDiagnosticsHandler {
	return &ToolDiagnosticsHandler{invocations: invocations}
}

// Get returns live per-user aggregates plus bounded durable invocation summaries.
func (h *ToolDiagnosticsHandler) Get(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 200 {
			respondError(w, http.StatusBadRequest, "limit must be an integer between 1 and 200")
			return
		}
		limit = parsed
	}

	userID := auth.ScopeUserIDFromContext(r.Context())
	items, err := h.invocations.ListForUser(userID, repository.ToolInvocationListOptions{
		Limit:    limit,
		ToolName: r.URL.Query().Get("tool_name"),
		Status:   r.URL.Query().Get("status"),
	})
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"scope":       "user",
		"metrics":     tools.ToolMetricsSnapshotForUser(userID),
		"invocations": items,
	})
}
