package tools

import (
	"context"
	"fmt"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

func sandboxOwnerFromContext(ctx context.Context) (sandbox.OwnerScope, error) {
	scope := InvocationScopeFromContext(ctx)
	if scope.UserID == "" {
		return sandbox.OwnerScope{}, fmt.Errorf("sandbox execution requires an authenticated invocation owner")
	}
	return sandbox.OwnerScope{
		UserID:         scope.UserID,
		WorkspaceID:    scope.WorkspaceID,
		ConversationID: scope.ConversationID,
		MessageID:      scope.MessageID,
		AgentRunID:     scope.RunID,
	}, nil
}

func defaultCodeSandboxSpec(timeoutMS int) sandbox.CreateRequest {
	if timeoutMS <= 0 {
		timeoutMS = 30000
	}
	return sandbox.CreateRequest{
		Network: sandbox.NetworkPolicy{Mode: sandbox.NetworkNone},
		Resources: sandbox.ResourceLimits{
			WallTimeMS:      timeoutMS,
			MaxStdoutBytes:  1 << 20,
			MaxStderrBytes:  1 << 20,
			MaxArtifactBytes: 8 << 20,
		},
		Requirements: sandbox.RuntimeRequirements{
			OSIsolation:          true,
			FilesystemIsolation:  true,
			NetworkIsolation:     true,
			ProcessTreeIsolation: true,
		},
	}
}
