package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

// Registry holds all registered tools and supports thread-safe lookup. An
// optional policy resolver can additionally hide hard-denied tools from runtime
// discovery without removing them from the registry or Settings inventory.
type Registry struct {
	mu                   sync.RWMutex
	tools                map[string]Tool
	policyResolver       PermissionResolver
	scopedPolicyResolver func(context.Context, string) string
}

// NewRegistry creates a registry with dependency-free core utilities. Tools
// requiring application services are still registered by api/router.go. Local
// Git read tools are added when repository IDs are configured; mutation tools
// additionally require OMNILLM_GIT_WRITE_ENABLED=true. Remote Git inspection
// additionally requires configured remotes and OMNILLM_GIT_REMOTE_ENABLED=true;
// fetch also requires the local write gate; push and branch publication have
// their own additional mutation gates; clone requires explicit enablement plus
// valid byte/entry budgets. GitHub PR reads (including review-thread state),
// draft creation, review replies, review-thread resolution, and draft-to-ready
// transition are independent API gates and none implies Git push or another
// hosted gate.
func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	r.MustRegister(NewDateTimeTool())
	r.MustRegister(NewUnitConvertTool())
	r.MustRegister(NewPythonAnalysisTool())
	for _, tool := range NewWorkspaceTools() {
		r.MustRegister(tool)
	}
	gitService := gitrepo.NewServiceFromEnvironment()
	if gitService.Configured() {
		for _, tool := range NewGitRepositoryTools(gitService) {
			r.MustRegister(tool)
		}
		if gitService.WriteEnabled() {
			for _, tool := range NewGitRepositoryMutationTools(gitService) {
				r.MustRegister(tool)
			}
		}
		remoteGitService := gitrepo.NewRemoteServiceFromEnvironment(gitService)
		if remoteGitService.Configured() && remoteGitService.Enabled() {
			for _, tool := range NewGitRemoteTools(remoteGitService) {
				r.MustRegister(tool)
			}
			if remoteGitService.FetchEnabled() {
				for _, tool := range NewGitRemoteMutationTools(remoteGitService) {
					r.MustRegister(tool)
				}
			}
			if remoteGitService.PushMutationEnabled() {
				for _, tool := range NewGitRemotePushTools(remoteGitService) {
					r.MustRegister(tool)
				}
			}
			if remoteGitService.GitHubPullRequestReadAccessEnabled() {
				for _, tool := range NewGitHubPullRequestReadTools(remoteGitService) {
					r.MustRegister(tool)
				}
				for _, tool := range NewGitHubPullRequestReviewThreadTools(remoteGitService) {
					r.MustRegister(tool)
				}
			}
			if remoteGitService.GitHubPullRequestMutationEnabled() {
				for _, tool := range NewGitHubPullRequestTools(remoteGitService) {
					r.MustRegister(tool)
				}
			}
			if remoteGitService.GitHubPullRequestReplyMutationEnabled() {
				for _, tool := range NewGitHubPullRequestReplyTools(remoteGitService) {
					r.MustRegister(tool)
				}
			}
			if remoteGitService.GitHubPullRequestThreadResolutionMutationEnabled() {
				for _, tool := range NewGitHubPullRequestReviewThreadResolutionTools(remoteGitService) {
					r.MustRegister(tool)
				}
			}
			if remoteGitService.GitHubPullRequestReadyMutationEnabled() {
				for _, tool := range NewGitHubPullRequestReadyTools(remoteGitService) {
					r.MustRegister(tool)
				}
			}
			if remoteGitService.CloneMutationEnabled() {
				for _, tool := range NewGitRemoteCloneTools(remoteGitService) {
					r.MustRegister(tool)
				}
			}
		}
	}
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
// was found. Get deliberately ignores runtime policy so the executor and
// Settings endpoints can still inspect a denied tool and report why it cannot
// run.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// SetPolicyResolver configures the effective runtime policy used by discovery.
// Hard-denied tools remain registered but are excluded from ListEnabled and
// Select. Passing nil restores definition-only discovery behavior.
func (r *Registry) SetPolicyResolver(resolver PermissionResolver) {
	r.mu.Lock()
	r.policyResolver = resolver
	r.mu.Unlock()
}

// Policy returns the current runtime policy exposed to discovery. An empty
// string means no policy resolver is attached to the registry.
func (r *Registry) Policy(name string) string {
	r.mu.RLock()
	resolver := r.policyResolver
	r.mu.RUnlock()
	if resolver == nil {
		return ""
	}
	return resolver(name)
}

// IsAvailable reports whether a tool is statically enabled and not hard-denied
// by the attached runtime policy.
func (r *Registry) IsAvailable(name string) bool {
	tool, ok := r.Get(name)
	if !ok {
		return false
	}
	definition := tool.Definition().Normalized()
	if !definition.Enabled {
		return false
	}
	return r.Policy(name) != "deny"
}

// SetScopedPolicyResolver binds request-scoped effective policy for discovery.
func (r *Registry) SetScopedPolicyResolver(resolver func(context.Context, string) string) {
	r.mu.Lock()
	r.scopedPolicyResolver = resolver
	r.mu.Unlock()
}

// PolicyForContext returns request-scoped policy when configured.
func (r *Registry) PolicyForContext(ctx context.Context, name string) string {
	r.mu.RLock()
	resolver := r.scopedPolicyResolver
	r.mu.RUnlock()
	if resolver != nil {
		return resolver(ctx, name)
	}
	return r.Policy(name)
}

// IsAvailableForContext applies static enabled-state and request-scoped hard deny.
func (r *Registry) IsAvailableForContext(ctx context.Context, name string) bool {
	tool, ok := r.Get(name)
	if !ok || !tool.Definition().Normalized().Enabled {
		return false
	}
	return r.PolicyForContext(ctx, name) != "deny"
}

// ListEnabledForContext returns enabled definitions that are not request-scoped denied.
func (r *Registry) ListEnabledForContext(ctx context.Context) []ToolDefinition {
	defs := r.ListEnabled()
	out := make([]ToolDefinition, 0, len(defs))
	for _, def := range defs {
		if r.PolicyForContext(ctx, def.Name) != "deny" {
			out = append(out, def)
		}
	}
	return out
}

// SelectForContext is Select with request-scoped deny filtering.
func (r *Registry) SelectForContext(ctx context.Context, terms []string, limit int) []ToolDefinition {
	if limit <= 0 {
		return nil
	}
	candidates := r.Select(terms, limit*2+8)
	out := make([]ToolDefinition, 0, limit)
	for _, def := range candidates {
		if r.PolicyForContext(ctx, def.Name) == "deny" {
			continue
		}
		out = append(out, def)
		if len(out) >= limit {
			break
		}
	}
	return out
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

// ListEnabled returns normalized definitions only for tools that are statically
// enabled and not hard-denied by the attached runtime policy.
func (r *Registry) ListEnabled() []ToolDefinition {
	defs := r.List()
	r.mu.RLock()
	resolver := r.policyResolver
	r.mu.RUnlock()

	out := make([]ToolDefinition, 0, len(defs))
	for _, definition := range defs {
		if !definition.Enabled {
			continue
		}
		if resolver != nil && resolver(definition.Name) == "deny" {
			continue
		}
		out = append(out, definition)
	}
	return out
}

// Select returns available tools whose names, descriptions, or categories match
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
