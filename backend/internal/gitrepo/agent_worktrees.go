package gitrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/uuid"
)

const AgentWorktreeRootEnv = "OMNILLM_AGENT_WORKTREE_ROOT"

// AgentWorktreeOwner is the durable application authority attached to one
// isolated worktree. IDs are compared exactly; none is interpreted as a path.
type AgentWorktreeOwner struct {
	UserID         string `json:"user_id"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	AgentRunID     string `json:"agent_run_id"`
}

func (o AgentWorktreeOwner) valid() bool {
	return strings.TrimSpace(o.UserID) != "" && strings.TrimSpace(o.AgentRunID) != ""
}

type AgentWorktree struct {
	ID         string             `json:"id"`
	Repository string             `json:"repository"`
	Owner      AgentWorktreeOwner `json:"owner"`
	BaseCommit string             `json:"base_commit"`
	CreatedAt  time.Time          `json:"created_at"`
}

type AgentWorktreeReview struct {
	WorktreeID  string `json:"worktree_id"`
	BaseCommit  string `json:"base_commit"`
	Patch       string `json:"patch"`
	PatchSHA256 string `json:"patch_sha256"`
}

// AgentWorktreeManager owns generated immutable-base + writable-worktree
// snapshots below an operator-chosen root. It deliberately does not create a
// linked Git worktree: repository hooks, checkout filters, credential helpers,
// and .git authority therefore never enter the sandbox-visible filesystem or
// execute while the snapshot is materialized.
type AgentWorktreeManager struct {
	service *Service
	root    string
	gitPath string
}

func NewAgentWorktreeManager(service *Service, root string) (*AgentWorktreeManager, error) {
	if service == nil || !service.Configured() {
		return nil, fmt.Errorf("configured git service is required")
	}
	if !service.WriteEnabled() {
		return nil, errWriteDisabled
	}
	root = strings.TrimSpace(root)
	if root == "" {
		root = strings.TrimSpace(os.Getenv(AgentWorktreeRootEnv))
	}
	if root == "" {
		return nil, fmt.Errorf("agent worktree root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve agent worktree root: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("create agent worktree root: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = filepath.Clean(resolved)
	}
	for _, repoRoot := range service.repositories {
		if pathWithin(repoRoot, absolute) || pathWithin(absolute, repoRoot) {
			return nil, fmt.Errorf("agent worktree root must be separate from configured repository roots")
		}
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("find git executable: %w", err)
	}
	return &AgentWorktreeManager{service: service, root: absolute, gitPath: gitPath}, nil
}

func pathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (m *AgentWorktreeManager) Create(ctx context.Context, repositoryID string, owner AgentWorktreeOwner, baseRef string) (*AgentWorktree, error) {
	if m == nil || m.service == nil {
		return nil, fmt.Errorf("agent worktree manager is unavailable")
	}
	if !owner.valid() {
		return nil, fmt.Errorf("agent worktree requires user_id and agent_run_id")
	}
	repositoryID = strings.TrimSpace(repositoryID)
	repoRoot, ok := m.service.repositories[repositoryID]
	if !ok {
		return nil, fmt.Errorf("repository %q is not configured", repositoryID)
	}
	baseRef = strings.TrimSpace(baseRef)
	if baseRef == "" {
		baseRef = "HEAD"
	}
	if len(baseRef) > 256 || strings.HasPrefix(baseRef, "-") || strings.ContainsAny(baseRef, "\x00\r\n") {
		return nil, fmt.Errorf("invalid agent worktree base revision")
	}

	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return nil, safeRepositoryError(repositoryID, "could not be opened")
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(baseRef))
	if err != nil || hash == nil {
		return nil, fmt.Errorf("resolve agent worktree base revision")
	}
	commit, err := repo.CommitObject(*hash)
	if err != nil {
		return nil, fmt.Errorf("agent worktree base revision is not a commit")
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("read agent worktree base tree")
	}

	id := "awt_" + uuid.NewString()
	container := filepath.Join(m.root, id)
	basePath := filepath.Join(container, "base")
	worktreePath := filepath.Join(container, "worktree")
	if err := os.Mkdir(container, 0o700); err != nil {
		return nil, fmt.Errorf("create agent worktree container: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(container)
		}
	}()
	if err := materializeAgentTree(ctx, tree, basePath); err != nil {
		return nil, fmt.Errorf("materialize agent worktree base: %w", err)
	}
	if err := materializeAgentTree(ctx, tree, worktreePath); err != nil {
		return nil, fmt.Errorf("materialize agent worktree: %w", err)
	}
	created := &AgentWorktree{
		ID:         id,
		Repository: repositoryID,
		Owner:      owner,
		BaseCommit: commit.Hash.String(),
		CreatedAt:  time.Now().UTC(),
	}
	if err := writeAgentWorktreeMetadata(filepath.Join(container, "metadata.json"), created); err != nil {
		return nil, err
	}
	cleanup = false
	return created, nil
}

func materializeAgentTree(ctx context.Context, tree *object.Tree, destination string) error {
	if tree == nil {
		return fmt.Errorf("git tree is required")
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		return err
	}
	iter := tree.Files()
	defer iter.Close()
	return iter.ForEach(func(file *object.File) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		clean, err := cleanRepositoryPath(file.Name)
		if err != nil {
			return fmt.Errorf("base tree contains an unsafe path")
		}
		target := filepath.Join(destination, clean)
		if !pathWithin(destination, target) {
			return fmt.Errorf("base tree path escapes snapshot")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		switch file.Mode {
		case filemode.Regular, filemode.Deprecated:
			return copyAgentBlob(file, target, 0o600)
		case filemode.Executable:
			return copyAgentBlob(file, target, 0o700)
		case filemode.Symlink:
			contents, err := file.Contents()
			if err != nil {
				return err
			}
			linkTarget := strings.TrimSpace(contents)
			if linkTarget == "" || filepath.IsAbs(linkTarget) || filepath.VolumeName(linkTarget) != "" {
				return fmt.Errorf("base tree contains an unsafe symlink")
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(target), filepath.FromSlash(linkTarget)))
			if !pathWithin(destination, resolved) {
				return fmt.Errorf("base tree symlink escapes snapshot")
			}
			return os.Symlink(filepath.FromSlash(linkTarget), target)
		default:
			return fmt.Errorf("base tree contains unsupported file mode %v", file.Mode)
		}
	})
}

func copyAgentBlob(file *object.File, target string, mode os.FileMode) error {
	reader, err := file.Reader()
	if err != nil {
		return err
	}
	defer reader.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, reader)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func writeAgentWorktreeMetadata(path string, value *AgentWorktree) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write agent worktree metadata: %w", err)
	}
	return nil
}

func (m *AgentWorktreeManager) load(id string, owner AgentWorktreeOwner) (*AgentWorktree, string, string, error) {
	id = strings.TrimSpace(id)
	if !strings.HasPrefix(id, "awt_") || strings.ContainsAny(id, "/\\\x00") {
		return nil, "", "", fmt.Errorf("invalid agent worktree id")
	}
	container := filepath.Join(m.root, id)
	if !pathWithin(m.root, container) {
		return nil, "", "", fmt.Errorf("invalid agent worktree path")
	}
	encoded, err := os.ReadFile(filepath.Join(container, "metadata.json"))
	if err != nil {
		return nil, "", "", fmt.Errorf("agent worktree not found")
	}
	var value AgentWorktree
	if err := json.Unmarshal(encoded, &value); err != nil {
		return nil, "", "", fmt.Errorf("agent worktree metadata is invalid")
	}
	if value.ID != id || value.Owner != owner || !owner.valid() {
		return nil, "", "", fmt.Errorf("agent worktree is not owned by the current scope")
	}
	if _, ok := m.service.repositories[value.Repository]; !ok {
		return nil, "", "", fmt.Errorf("agent worktree repository is no longer configured")
	}
	basePath := filepath.Join(container, "base")
	worktreePath := filepath.Join(container, "worktree")
	for _, candidate := range []string{basePath, worktreePath} {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil || filepath.Clean(resolved) != filepath.Clean(candidate) {
			return nil, "", "", fmt.Errorf("agent worktree path identity changed")
		}
	}
	return &value, basePath, worktreePath, nil
}

// InternalPath returns the generated physical writable path for trusted
// workspace registration. Callers must never expose it in model arguments.
func (m *AgentWorktreeManager) InternalPath(id string, owner AgentWorktreeOwner) (string, error) {
	_, _, path, err := m.load(id, owner)
	return path, err
}

// Review compares the immutable base snapshot with the sandbox-visible copy.
// The diff command runs outside any repository, with external diff/textconv
// disabled, so repository-local Git configuration cannot execute code.
func (m *AgentWorktreeManager) Review(ctx context.Context, id string, owner AgentWorktreeOwner) (*AgentWorktreeReview, error) {
	value, basePath, worktreePath, err := m.load(id, owner)
	if err != nil {
		return nil, err
	}
	patch, err := m.diffNoIndex(ctx, filepath.Dir(basePath), filepath.Base(basePath), filepath.Base(worktreePath))
	if err != nil {
		return nil, fmt.Errorf("review agent worktree: %w", err)
	}
	patch = normalizeAgentPatchPaths(patch)
	digest := sha256.Sum256([]byte(patch))
	return &AgentWorktreeReview{
		WorktreeID:  id,
		BaseCommit:  value.BaseCommit,
		Patch:       patch,
		PatchSHA256: fmt.Sprintf("%x", digest[:]),
	}, nil
}

func normalizeAgentPatchPaths(patch string) string {
	replacements := [][2]string{
		{"a/base/", "a/"},
		{"a/worktree/", "a/"},
		{"b/base/", "b/"},
		{"b/worktree/", "b/"},
	}
	for _, replacement := range replacements {
		patch = strings.ReplaceAll(patch, replacement[0], replacement[1])
	}
	return patch
}

// Promote revalidates the exact review digest and immutable base commit before
// applying that patch into a clean configured base worktree. It does not stage,
// commit, fetch, push, or merge; the existing guarded Git tools remain
// authoritative for publication.
func (m *AgentWorktreeManager) Promote(ctx context.Context, id string, owner AgentWorktreeOwner, expectedPatchSHA256 string) error {
	value, _, _, err := m.load(id, owner)
	if err != nil {
		return err
	}
	review, err := m.Review(ctx, id, owner)
	if err != nil {
		return err
	}
	if strings.TrimSpace(expectedPatchSHA256) == "" || !strings.EqualFold(review.PatchSHA256, strings.TrimSpace(expectedPatchSHA256)) {
		return fmt.Errorf("agent worktree changed since review")
	}
	repo, err := m.service.open(value.Repository)
	if err != nil {
		return err
	}
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("inspect promotion target HEAD")
	}
	if !strings.EqualFold(head.Hash().String(), value.BaseCommit) {
		return fmt.Errorf("promotion target HEAD changed since agent worktree creation")
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return safeRepositoryError(value.Repository, "does not expose a worktree")
	}
	status, err := worktree.Status()
	if err != nil {
		return fmt.Errorf("inspect promotion target")
	}
	if !status.IsClean() {
		return fmt.Errorf("promotion target worktree must be clean")
	}
	if strings.TrimSpace(review.Patch) == "" {
		return nil
	}
	repoRoot := m.service.repositories[value.Repository]
	if err := m.gitInput(ctx, repoRoot, []byte(review.Patch), "apply", "--check", "--binary", "-"); err != nil {
		return fmt.Errorf("agent worktree promotion check failed: %w", err)
	}
	if err := m.gitInput(ctx, repoRoot, []byte(review.Patch), "apply", "--binary", "-"); err != nil {
		return fmt.Errorf("apply reviewed agent worktree patch: %w", err)
	}
	return nil
}

func (m *AgentWorktreeManager) Remove(_ context.Context, id string, owner AgentWorktreeOwner) error {
	_, basePath, _, err := m.load(id, owner)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Dir(basePath)); err != nil {
		return fmt.Errorf("remove agent worktree: %w", err)
	}
	return nil
}

func (m *AgentWorktreeManager) diffNoIndex(ctx context.Context, directory, baseName, worktreeName string) (string, error) {
	args := []string{"diff", "--no-index", "--binary", "--no-ext-diff", "--no-textconv", "--", baseName, worktreeName}
	cmd := exec.CommandContext(ctx, m.gitPath, args...)
	cmd.Dir = directory
	cmd.Env = m.gitEnvironment()
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return string(output), nil
	}
	return "", fmt.Errorf("git diff failed")
}

func (m *AgentWorktreeManager) gitInput(ctx context.Context, directory string, input []byte, args ...string) error {
	global := []string{
		"-c", "core.hooksPath=" + filepath.Join(m.root, ".disabled-hooks"),
		"-c", "credential.helper=",
		"-c", "core.askPass=",
		"-C", directory,
	}
	cmd := exec.CommandContext(ctx, m.gitPath, append(global, args...)...)
	cmd.Env = m.gitEnvironment()
	cmd.Stdin = bytes.NewReader(input)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git command failed")
	}
	return nil
}

func (m *AgentWorktreeManager) gitEnvironment() []string {
	env := []string{
		"PATH=" + filepath.Dir(m.gitPath),
		"HOME=" + os.TempDir(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
	}
	for _, key := range []string{"SYSTEMROOT", "WINDIR", "PATHEXT", "TEMP", "TMP"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}
