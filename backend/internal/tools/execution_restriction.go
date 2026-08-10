package tools

import "context"

type executionRestriction struct {
	active bool
	allow  map[string]struct{}
}

type executionRestrictionContextKey struct{}

// ContextWithToolRestriction narrows execution to the supplied tool names.
// An active restriction with an empty set means no tools may execute. Nested
// restrictions intersect, so a child orchestration layer can never widen its parent.
func ContextWithToolRestriction(ctx context.Context, names []string) context.Context {
	next := executionRestriction{active: true, allow: make(map[string]struct{}, len(names))}
	for _, name := range names {
		if name != "" {
			next.allow[name] = struct{}{}
		}
	}
	if current, ok := ctx.Value(executionRestrictionContextKey{}).(executionRestriction); ok && current.active {
		for name := range next.allow {
			if _, allowed := current.allow[name]; !allowed {
				delete(next.allow, name)
			}
		}
	}
	return context.WithValue(ctx, executionRestrictionContextKey{}, next)
}

// ToolAllowedByContext reports whether a request-scoped restriction permits a tool.
func ToolAllowedByContext(ctx context.Context, name string) bool {
	if ctx == nil {
		return true
	}
	restriction, ok := ctx.Value(executionRestrictionContextKey{}).(executionRestriction)
	if !ok || !restriction.active {
		return true
	}
	_, ok = restriction.allow[name]
	return ok
}
