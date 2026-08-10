package tools

import "context"

// ScopedPermissionResolver composes request-specific user/workspace/conversation
// policy with the already-resolved global policy.
type ScopedPermissionResolver func(scope InvocationScope, toolName, basePolicy string) string

// SetScopedPermissionResolver enables request-scoped policy composition and binds
// the same view to Registry discovery/planning helpers.
func (e *Executor) SetScopedPermissionResolver(resolver ScopedPermissionResolver) {
	if e == nil {
		return
	}
	e.scopedPermissions = resolver
	if e.registry != nil {
		e.registry.SetScopedPolicyResolver(e.PolicyForContext)
	}
}

// PolicyForContext returns global effective policy narrowed by request scope.
func (e *Executor) PolicyForContext(ctx context.Context, name string) string {
	base := e.Policy(name)
	if e == nil || e.scopedPermissions == nil {
		return base
	}
	resolved := e.scopedPermissions(InvocationScopeFromContext(ctx), name, base)
	switch resolved {
	case "allow", "ask", "deny":
		return resolved
	default:
		return "deny"
	}
}
