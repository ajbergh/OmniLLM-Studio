package tools

// Definition returns a normalized registered tool definition.
func (e *Executor) Definition(name string) (ToolDefinition, bool) {
	if e == nil || e.registry == nil {
		return ToolDefinition{}, false
	}
	tool, ok := e.registry.Get(name)
	if !ok {
		return ToolDefinition{}, false
	}
	return tool.Definition().Normalized(), true
}

// EffectivePolicy resolves a stored allow/deny/ask value against the tool's
// definition. Missing or invalid persisted values use the same safe default the
// Settings API presents: consequential tools require approval; ordinary
// read-only tools are allowed.
func EffectivePolicy(definition ToolDefinition, stored string) string {
	switch stored {
	case "allow", "deny", "ask":
		return stored
	}

	definition = definition.Normalized()
	if definition.SideEffecting || definition.Risk == RiskHigh || definition.Risk == RiskCritical {
		return "ask"
	}
	return "allow"
}

// Policy returns the effective persisted policy for a registered tool. Unknown
// tools are denied. Missing resolver rows use EffectivePolicy so discovery,
// Settings, planning, and execution can share one default-policy contract.
func (e *Executor) Policy(name string) string {
	definition, ok := e.Definition(name)
	if !ok {
		return "deny"
	}

	stored := ""
	if e.permissions != nil {
		stored = e.permissions(name)
	}
	return EffectivePolicy(definition, stored)
}
