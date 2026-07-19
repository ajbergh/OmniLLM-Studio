package tools

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry holds all registered tools and supports thread-safe lookup.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates a registry with dependency-free core utilities. Tools
// requiring application services are still registered by api/router.go.
func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	r.MustRegister(NewDateTimeTool())
	r.MustRegister(NewUnitConvertTool())
	r.MustRegister(NewPythonAnalysisTool())
	return r
}

// Register adds a tool to the registry. Returns an error if a tool with the same
// name is already registered.
func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}
	def := tool.Definition().Normalized()
	if strings.TrimSpace(def.Name) == "" {
		return fmt.Errorf("tool name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[def.Name]; exists {
		return fmt.Errorf("tool %q already registered", def.Name)
	}
	r.tools[def.Name] = tool
	return nil
}

// MustRegister is like Register but panics on error.
func (r *Registry) MustRegister(tool Tool) {
	if err := r.Register(tool); err != nil {
		panic(err)
	}
}

// Remove unregisters a tool by name. It returns true when a tool was removed.
func (r *Registry) Remove(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; !exists {
		return false
	}
	delete(r.tools, name)
	return true
}

// Get returns a tool by name. The second return value indicates whether the tool
// was found.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns normalized definitions of all registered tools in stable order.
func (r *Registry) List() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition().Normalized())
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs
}

// ListEnabled returns normalized definitions only for tools that are enabled.
func (r *Registry) ListEnabled() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var defs []ToolDefinition
	for _, t := range r.tools {
		d := t.Definition().Normalized()
		if d.Enabled {
			defs = append(defs, d)
		}
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs
}

// Select returns enabled tools whose names, descriptions, or categories match
// one of the supplied terms. It provides a deterministic low-cost retrieval
// fallback until semantic tool embeddings are configured.
func (r *Registry) Select(terms []string, limit int) []ToolDefinition {
	if limit <= 0 {
		limit = 12
	}
	normalizedTerms := make([]string, 0, len(terms))
	for _, term := range terms {
		if term = strings.TrimSpace(strings.ToLower(term)); term != "" {
			normalizedTerms = append(normalizedTerms, term)
		}
	}
	defs := r.ListEnabled()
	if len(normalizedTerms) == 0 || len(defs) <= limit {
		if len(defs) > limit {
			return defs[:limit]
		}
		return defs
	}

	type scored struct {
		def   ToolDefinition
		score int
	}
	matches := make([]scored, 0, len(defs))
	for _, def := range defs {
		haystack := strings.ToLower(def.Name + " " + def.Description + " " + def.Category)
		score := 0
		for _, term := range normalizedTerms {
			if strings.Contains(strings.ToLower(def.Name), term) {
				score += 4
			}
			if strings.Contains(strings.ToLower(def.Category), term) {
				score += 2
			}
			if strings.Contains(haystack, term) {
				score++
			}
		}
		if score > 0 {
			matches = append(matches, scored{def: def, score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].def.Name < matches[j].def.Name
		}
		return matches[i].score > matches[j].score
	})
	out := make([]ToolDefinition, 0, min(limit, len(matches)))
	for i := 0; i < len(matches) && i < limit; i++ {
		out = append(out, matches[i].def)
	}
	return out
}

// Names returns tool names in stable order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
