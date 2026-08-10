package api

// chatCapabilityPolicy returns the current effective policy for a registered
// Chat Studio capability. Missing runtime dependencies fail closed.
func (h *MessageHandler) chatCapabilityPolicy(toolName string) string {
	if h == nil || h.toolExecutor == nil {
		return "deny"
	}
	return h.toolExecutor.Policy(toolName)
}

// chatPreflightAllowed is the single guard for deterministic Chat Studio
// capability shortcuts. Ask is intentionally not executable here: the request
// falls through to the normal tool loop so the shared approval flow can run.
func (h *MessageHandler) chatPreflightAllowed(toolName string) bool {
	return h.chatCapabilityPolicy(toolName) == "allow"
}

// chatCapabilityDiscoverable controls capability advertising in model context.
// Denied, disabled, or unknown tools must not be described to the model.
func (h *MessageHandler) chatCapabilityDiscoverable(toolName string) bool {
	if h == nil || h.toolRegistry == nil {
		return false
	}
	return h.toolRegistry.IsAvailable(toolName)
}
