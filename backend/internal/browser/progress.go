package browser

import "context"

type progressContextKey struct{}
type providerTypeContextKey struct{}

// ProgressFunc emits browser progress events to the caller, usually SSE.
type ProgressFunc func(event string, payload any)

// WithProgress attaches a browser progress callback to a context.
func WithProgress(ctx context.Context, fn ProgressFunc) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, progressContextKey{}, fn)
}

// ProgressFromContext returns the browser progress callback, if present.
func ProgressFromContext(ctx context.Context) ProgressFunc {
	fn, _ := ctx.Value(progressContextKey{}).(ProgressFunc)
	return fn
}

// WithProviderType attaches the active LLM provider type for browser result
// sizing decisions.
func WithProviderType(ctx context.Context, providerType string) context.Context {
	if providerType == "" {
		return ctx
	}
	return context.WithValue(ctx, providerTypeContextKey{}, providerType)
}

// ProviderTypeFromContext returns the active provider type, if present.
func ProviderTypeFromContext(ctx context.Context) string {
	v, _ := ctx.Value(providerTypeContextKey{}).(string)
	return v
}

func emitProgress(ctx context.Context, event string, payload any) {
	if fn := ProgressFromContext(ctx); fn != nil {
		fn(event, payload)
	}
}
