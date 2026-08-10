package gitrepo

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

func TestRemoteBranchCreateRequiresIndependentGlobalGate(t *testing.T) {
	configured := map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://example.com/repo.git", AllowPush: true, AllowBranchCreate: true},
	}
	writable := NewServiceWithWriteAccess(map[string]string{"repo": t.TempDir()}, true)
	readOnly := NewService(map[string]string{"repo": t.TempDir()})

	cases := []struct {
		name         string
		remote       bool
		push         bool
		branchCreate bool
		local        *Service
		want         bool
	}{
		{name: "all enabled", remote: true, push: true, branchCreate: true, local: writable, want: true},
		{name: "remote disabled", push: true, branchCreate: true, local: writable},
		{name: "push disabled", remote: true, branchCreate: true, local: writable},
		{name: "branch create disabled", remote: true, push: true, local: writable},
		{name: "local write disabled", remote: true, push: true, branchCreate: true, local: readOnly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newRemoteService(configured, tc.remote, tc.push, nil, nil)
			svc.local = tc.local
			svc.branchCreateEnabled = tc.branchCreate
			if got := svc.BranchCreateMutationEnabled(); got != tc.want {
				t.Fatalf("BranchCreateMutationEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRemoteConfigRequiresAllowPushForBranchCreate(t *testing.T) {
	configured := ParseRemoteConfig(`{
		"bad":{"repository":"repo","url":"https://example.com/repo.git","allow_branch_create":true},
		"good":{"repository":"repo","url":"https://example.com/repo.git","allow_push":true,"allow_branch_create":true}
	}`)
	if _, ok := configured["bad"]; ok {
		t.Fatal("branch-create opt-in accepted without allow_push")
	}
	if good, ok := configured["good"]; !ok || !good.AllowBranchCreate {
		t.Fatalf("valid branch-create config rejected: %#v", good)
	}
}

func TestRemoteBranchStateDigestBindsOnlyBranchNamespace(t *testing.T) {
	mainHash := plumbing.NewHash("1111111111111111111111111111111111111111")
	featureHash := plumbing.NewHash("2222222222222222222222222222222222222222")
	tagHash := plumbing.NewHash("3333333333333333333333333333333333333333")
	advertised := packp.NewAdvRefs()
	advertised.References["refs/heads/main"] = mainHash
	advertised.References["refs/heads/feature/a"] = featureHash
	advertised.References["refs/tags/v1"] = tagHash

	digest := remoteBranchStateDigest(advertised)
	if !validRemoteStateDigest(digest) {
		t.Fatalf("invalid branch-state digest %q", digest)
	}
	advertised.References["refs/tags/v1"] = plumbing.NewHash("4444444444444444444444444444444444444444")
	if got := remoteBranchStateDigest(advertised); got != digest {
		t.Fatalf("tag-only change altered branch digest: %q != %q", got, digest)
	}
	advertised.References["refs/heads/feature/a"] = plumbing.NewHash("5555555555555555555555555555555555555555")
	if got := remoteBranchStateDigest(advertised); got == digest {
		t.Fatal("branch-head change did not alter branch digest")
	}
}

func TestPublishBranchUsesZeroHashCreateCAS(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("base.txt"); err != nil {
		t.Fatal(err)
	}
	base, err := worktree.Commit("base", &git.CommitOptions{Author: &object.Signature{
		Name: "Publish Test", Email: "publish-test@example.invalid", When: time.Unix(1_700_000_000, 0).UTC(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	branchRef := plumbing.NewBranchReferenceName("feature/publish")
	if err := worktree.Checkout(&git.CheckoutOptions{Branch: branchRef, Create: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("feature.txt"); err != nil {
		t.Fatal(err)
	}
	head, err := worktree.Commit("feature", &git.CommitOptions{Author: &object.Signature{
		Name: "Publish Test", Email: "publish-test@example.invalid", When: time.Unix(1_700_000_100, 0).UTC(),
	}})
	if err != nil {
		t.Fatal(err)
	}

	advertised := packp.NewAdvRefs()
	advertised.References["refs/heads/main"] = base
	advertised.Head = &base
	transport := &publishTestTransport{session: &publishTestReceiveSession{advertised: advertised}}
	local := NewServiceWithWriteAccess(map[string]string{"repo": repoDir}, true)
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://example.com/repo.git", AllowPush: true, AllowBranchCreate: true},
	}, true, true, transport, nil)
	svc.local = local
	svc.branchCreateEnabled = true

	result, err := svc.PublishBranch(context.Background(), "origin", "feature/publish", head.String(), remoteBranchStateDigest(advertised))
	if err != nil {
		t.Fatalf("PublishBranch() returned error: %v", err)
	}
	if !result.Published || result.RemoteHead != head.String() || result.BytesSent <= 0 {
		t.Fatalf("unexpected publish result: %#v", result)
	}
	command := transport.session.command
	if command == nil {
		t.Fatal("receive-pack command was not captured")
	}
	if command.Name != branchRef || command.Old != plumbing.ZeroHash || command.New != head {
		t.Fatalf("publish command = %#v, want %s: zero -> %s", command, branchRef, head)
	}
	tracking, err := repo.Reference(remoteTrackingReference("origin", "feature/publish"), true)
	if err != nil || tracking.Hash() != head {
		t.Fatalf("tracking ref = %#v, %v; want %s", tracking, err, head)
	}
}

func TestPublishBranchRejectsStaleRemoteBranchState(t *testing.T) {
	svc, _, head, transport, advertised := newPublishBranchTestService(t, "feature/stale")
	staleDigest := strings.Repeat("a", 64)
	if staleDigest == remoteBranchStateDigest(advertised) {
		staleDigest = strings.Repeat("b", 64)
	}
	_, err := svc.PublishBranch(context.Background(), "origin", "feature/stale", head.String(), staleDigest)
	if err == nil || !strings.Contains(err.Error(), "remote branch state changed") {
		t.Fatalf("PublishBranch() error = %v, want stale remote-state rejection", err)
	}
	if transport.session.command != nil {
		t.Fatal("receive-pack command was sent after stale remote-state rejection")
	}
}

func TestPublishBranchRejectsExistingRemoteBranch(t *testing.T) {
	svc, _, head, transport, advertised := newPublishBranchTestService(t, "feature/existing")
	advertised.References[plumbing.NewBranchReferenceName("feature/existing").String()] = plumbing.NewHash("7777777777777777777777777777777777777777")
	digest := remoteBranchStateDigest(advertised)
	_, err := svc.PublishBranch(context.Background(), "origin", "feature/existing", head.String(), digest)
	if err == nil || !strings.Contains(err.Error(), "already advertises branch") {
		t.Fatalf("PublishBranch() error = %v, want existing-branch rejection", err)
	}
	if transport.session.command != nil {
		t.Fatal("receive-pack command was sent for an existing remote branch")
	}
}

func newPublishBranchTestService(t *testing.T, branch string) (*RemoteService, *git.Repository, plumbing.Hash, *publishTestTransport, *packp.AdvRefs) {
	t.Helper()
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("base.txt"); err != nil {
		t.Fatal(err)
	}
	base, err := worktree.Commit("base", &git.CommitOptions{Author: &object.Signature{
		Name: "Publish Test", Email: "publish-test@example.invalid", When: time.Unix(1_700_000_000, 0).UTC(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(branch), Create: true}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("feature.txt"); err != nil {
		t.Fatal(err)
	}
	head, err := worktree.Commit("feature", &git.CommitOptions{Author: &object.Signature{
		Name: "Publish Test", Email: "publish-test@example.invalid", When: time.Unix(1_700_000_100, 0).UTC(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	advertised := packp.NewAdvRefs()
	advertised.References["refs/heads/main"] = base
	advertised.Head = &base
	remoteTransport := &publishTestTransport{session: &publishTestReceiveSession{advertised: advertised}}
	local := NewServiceWithWriteAccess(map[string]string{"repo": repoDir}, true)
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://example.com/repo.git", AllowPush: true, AllowBranchCreate: true},
	}, true, true, remoteTransport, nil)
	svc.local = local
	svc.branchCreateEnabled = true
	return svc, repo, head, remoteTransport, advertised
}

type publishTestTransport struct {
	session *publishTestReceiveSession
}

func (t *publishTestTransport) NewUploadPackSession(*transport.Endpoint, transport.AuthMethod) (transport.UploadPackSession, error) {
	return nil, errors.New("upload-pack is not supported by publish test transport")
}

func (t *publishTestTransport) NewReceivePackSession(*transport.Endpoint, transport.AuthMethod) (transport.ReceivePackSession, error) {
	return t.session, nil
}

type publishTestReceiveSession struct {
	advertised *packp.AdvRefs
	command    *packp.Command
}

func (s *publishTestReceiveSession) AdvertisedReferences() (*packp.AdvRefs, error) {
	return s.advertised, nil
}

func (s *publishTestReceiveSession) AdvertisedReferencesContext(context.Context) (*packp.AdvRefs, error) {
	return s.advertised, nil
}

func (s *publishTestReceiveSession) Close() error { return nil }

func (s *publishTestReceiveSession) ReceivePack(_ context.Context, request *packp.ReferenceUpdateRequest) (*packp.ReportStatus, error) {
	if len(request.Commands) != 1 {
		return nil, errors.New("expected exactly one receive-pack command")
	}
	s.command = request.Commands[0]
	if request.Packfile != nil {
		if _, err := io.Copy(io.Discard, request.Packfile); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
