package tools

import "context"

type inlineApprovalContextKey struct{}

// ContextWithInlineApproval keeps an ordinary Chat Studio tool invocation on
// the original request while the user reviews an ask-policy decision.
func ContextWithInlineApproval(ctx context.Context) context.Context {
	return context.WithValue(ctx, inlineApprovalContextKey{}, true)
}

func inlineApprovalEnabled(ctx context.Context) bool {
	enabled, _ := ctx.Value(inlineApprovalContextKey{}).(bool)
	return enabled
}
