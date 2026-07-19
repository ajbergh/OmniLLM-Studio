package tools

// Definition returns a normalized registered tool definition.
func (e *Executor) Definition(name string) (ToolDefinition, bool) {
	tool, ok := e.registry.Get(name)
	if !ok {
		return ToolDefinition{}, false
	}
	return tool.Definition().Normalized(), true
}

// Policy returns the effective persisted policy for a tool. Legacy empty values
// are normalized to allow for backward compatibility.
func (e *Executor) Policy(name string) string {
	if e.permissions == nil {
		return "allow"
	}
	policy := e.permissions(name)
	if policy == "" {
		return "allow"
	}
	return policy
}
