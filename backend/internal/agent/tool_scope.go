package agent

import (
	"context"

	"github.com/ajbergh/omnillm-studio/internal/tools"
)

type allowedToolsContextKey struct{}

// ContextWithAllowedTools restricts an Agent run to a saved profile's tool set.
// A nil/empty list means unrestricted for backwards compatibility. The generic
// tool restriction is also propagated so nested orchestration cannot escape the
// Assistant Profile's allowed-tool boundary.
func ContextWithAllowedTools(ctx context.Context, names []string) context.Context {
	if len(names) == 0 {
		return ctx
	}
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name != "" {
			allowed[name] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return ctx
	}
	ctx = tools.ContextWithToolRestriction(ctx, names)
	return context.WithValue(ctx, allowedToolsContextKey{}, allowed)
}

func toolAllowedByContext(ctx context.Context, name string) bool {
	if ctx == nil {
		return true
	}
	allowed, ok := ctx.Value(allowedToolsContextKey{}).(map[string]struct{})
	if !ok || len(allowed) == 0 {
		return true
	}
	_, ok = allowed[name]
	return ok
}

func filterDefinitionsByContext(ctx context.Context, defs []interface{ GetName() string }) []interface{ GetName() string } {
	out := make([]interface{ GetName() string }, 0, len(defs))
	for _, def := range defs {
		if toolAllowedByContext(ctx, def.GetName()) {
			out = append(out, def)
		}
	}
	return out
}
