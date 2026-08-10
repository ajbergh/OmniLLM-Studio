package tools

import (
	"encoding/json"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

const gitToolResultLimit = 128 << 10

type gitRepositoryTool struct {
	service gitrepo.Reader
	name    string
}

// NewGitRepositoryTools returns the read-only local Git tool family backed by svc.
func NewGitRepositoryTools(svc gitrepo.Reader) []Tool {
	if svc == nil {
		return nil
	}
	names := []string{
		"git_repositories",
		"git_status",
		"git_diff",
		"git_log",
		"git_show",
		"git_branches",
		"git_blame",
	}
	out := make([]Tool, 0, len(names))
	for _, name := range names {
		out = append(out, &gitRepositoryTool{service: svc, name: name})
	}
	return out
}

func (t *gitRepositoryTool) Definition() ToolDefinition {
	definition := ToolDefinition{
		Name:             t.name,
		Category:         "git",
		Enabled:          true,
		Version:          "1",
		Risk:             RiskLow,
		ReadOnly:         true,
		SideEffecting:    false,
		RequiresNetwork:  false,
		SupportsParallel: true,
		DefaultTimeoutMS: 10_000,
		MaxResultBytes:   gitToolResultLimit,
	}

	switch t.name {
	case "git_repositories":
		definition.Description = "List explicitly configured local Git repositories by stable repository ID, including branch, HEAD, and clean/dirty status. Filesystem paths are never exposed."
		definition.Parameters = json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)
	case "git_status":
		definition.Description = "Read local Git status for a configured repository, including branch, HEAD, staged changes, unstaged changes, and untracked files."
		definition.Parameters = repositoryOnlySchema()
	case "git_diff":
		definition.Description = "Read a local Git diff. With only repository, compares the combined worktree against HEAD. With from/to revisions, compares committed revisions; to defaults to HEAD."
		definition.Parameters = json.RawMessage(`{
			"type":"object",
			"properties":{
				"repository":{"type":"string","description":"Configured repository ID from git_repositories"},
				"from":{"type":"string","description":"Optional source revision. Omit both from and to for worktree vs HEAD."},
				"to":{"type":"string","description":"Optional destination revision. Defaults to HEAD when from is provided."}
			},
			"required":["repository"],
			"additionalProperties":false
		}`)
	case "git_log":
		definition.Description = "Read recent commit history from a configured local Git repository."
		definition.Parameters = json.RawMessage(`{
			"type":"object",
			"properties":{
				"repository":{"type":"string","description":"Configured repository ID from git_repositories"},
				"revision":{"type":"string","description":"Revision to start from; defaults to HEAD"},
				"limit":{"type":"integer","minimum":1,"maximum":100,"description":"Maximum commits to return; defaults to 20"}
			},
			"required":["repository"],
			"additionalProperties":false
		}`)
	case "git_show":
		definition.Description = "Read metadata and full commit message for one revision in a configured local Git repository."
		definition.Parameters = json.RawMessage(`{
			"type":"object",
			"properties":{
				"repository":{"type":"string","description":"Configured repository ID from git_repositories"},
				"revision":{"type":"string","description":"Revision to inspect; defaults to HEAD"}
			},
			"required":["repository"],
			"additionalProperties":false
		}`)
	case "git_branches":
		definition.Description = "List local branches and identify the current branch or detached HEAD state for a configured local Git repository."
		definition.Parameters = repositoryOnlySchema()
	case "git_blame":
		definition.Description = "Read bounded line attribution for a committed file in a configured local Git repository. Paths must be repository-relative."
		definition.Parameters = json.RawMessage(`{
			"type":"object",
			"properties":{
				"repository":{"type":"string","description":"Configured repository ID from git_repositories"},
				"path":{"type":"string","description":"Repository-relative committed file path"},
				"revision":{"type":"string","description":"Revision to blame; defaults to HEAD"},
				"start_line":{"type":"integer","minimum":1,"description":"Optional first line, 1-based"},
				"end_line":{"type":"integer","minimum":1,"description":"Optional last line, 1-based"}
			},
			"required":["repository","path"],
			"additionalProperties":false
		}`)
	}
	return definition
}
