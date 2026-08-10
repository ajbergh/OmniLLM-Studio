package gitrepo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/revlist"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

func TestCloneLimitsRequireExplicitValidBudgets(t *testing.T) {
	t.Setenv(RemoteCloneMaxBytesEnv, "")
	t.Setenv(RemoteCloneMaxEntriesEnv, "")
	if _, _, ok := cloneLimitsFromEnvironment(); ok {
		t.Fatal("clone limits unexpectedly valid without explicit budgets")
	}

	t.Setenv(RemoteCloneMaxBytesEnv, "268435456")
	t.Setenv(RemoteCloneMaxEntriesEnv, "25000")
	bytes, entries, ok := cloneLimitsFromEnvironment()
	if !ok || bytes != 268435456 || entries != 25000 {
		t.Fatalf("clone limits = (%d, %d, %v), want (268435456, 25000, true)", bytes, entries, ok)
	}

	t.Setenv(RemoteCloneMaxBytesEnv, "1024")
	if _, _, ok := cloneLimitsFromEnvironment(); ok {
		t.Fatal("clone byte budget below hard minimum was accepted")
	}
	t.Setenv(RemoteCloneMaxBytesEnv, "268435456")
	t.Setenv(RemoteCloneMaxEntriesEnv, "100001")
	if _, _, ok := cloneLimitsFromEnvironment(); ok {
		t.Fatal("clone entry budget above hard maximum was accepted")
	}
}

func TestCloneMutationEnabledRequiresAllGlobalGatesAndBudgets(t *testing.T) {
	configured := map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://example.com/repo.git", AllowClone: true},
	}
	writable := NewServiceWithWriteAccess(map[string]string{"repo": filepath.Join(t.TempDir(), "repo")}, true)
	readOnly := NewService(map[string]string{"repo": filepath.Join(t.TempDir(), "repo")})

	cases := []struct {
		name       string
		enabled    bool
		cloneGate  bool
		maxBytes   int64
		maxEntries int64
		local      *Service
		want       bool
	}{
		{name: "all enabled", enabled: true, cloneGate: true, maxBytes: minRemoteCloneBytes, maxEntries: minRemoteCloneEntries, local: writable, want: true},
		{name: "remote disabled", cloneGate: true, maxBytes: minRemoteCloneBytes, maxEntries: minRemoteCloneEntries, local: writable},
		{name: "clone disabled", enabled: true, maxBytes: minRemoteCloneBytes, maxEntries: minRemoteCloneEntries, local: writable},
		{name: "missing bytes", enabled: true, cloneGate: true, maxEntries: minRemoteCloneEntries, local: writable},
		{name: "missing entries", enabled: true, cloneGate: true, maxBytes: minRemoteCloneBytes, local: writable},
		{name: "local write disabled", enabled: true, cloneGate: true, maxBytes: minRemoteCloneBytes, maxEntries: minRemoteCloneEntries, local: readOnly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newRemoteService(configured, tc.enabled, false, nil, nil)
			svc.local = tc.local
			svc.cloneEnabled = tc.cloneGate
			svc.cloneMaxBytes = tc.maxBytes
			svc.cloneMaxEntries = tc.maxEntries
			if got := svc.CloneMutationEnabled(); got != tc.want {
				t.Fatalf("CloneMutationEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCloneDestinationMustBeAbsentWithExistingRealParent(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "clone")
	local := NewServiceWithWriteAccess(map[string]string{"repo": destination}, true)
	svc := newRemoteService(nil, true, false, nil, nil)
	svc.local = local

	got, err := svc.cloneDestination("repo")
	if err != nil || got != destination {
		t.Fatalf("cloneDestination() = (%q, %v), want (%q, nil)", got, err, destination)
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.cloneDestination("repo"); err == nil {
		t.Fatal("existing destination unexpectedly accepted")
	}
}

func TestCloneDestinationRejectsMissingParent(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "missing", "clone")
	local := NewServiceWithWriteAccess(map[string]string{"repo": destination}, true)
	svc := newRemoteService(nil, true, false, nil, nil)
	svc.local = local
	if _, err := svc.cloneDestination("repo"); err == nil {
		t.Fatal("destination with missing parent unexpectedly accepted")
	}
}

func TestValidateClonePackHeader(t *testing.T) {
	valid := clonePackHeader(2, 42)
	if err := validateClonePackHeader(bufio.NewReader(bytes.NewReader(valid))); err != nil {
		t.Fatalf("valid pack header rejected: %v", err)
	}
	invalidMagic := append([]byte(nil), valid...)
	copy(invalidMagic[:4], []byte("NOPE"))
	if err := validateClonePackHeader(bufio.NewReader(bytes.NewReader(invalidMagic))); err == nil {
		t.Fatal("invalid pack magic accepted")
	}
	if err := validateClonePackHeader(bufio.NewReader(bytes.NewReader(clonePackHeader(4, 1)))); err == nil {
		t.Fatal("unsupported pack version accepted")
	}
	if err := validateClonePackHeader(bufio.NewReader(bytes.NewReader(clonePackHeader(2, maxRemoteCloneObjects+1)))); err == nil {
		t.Fatal("oversized pack object count accepted")
	}
}

func TestCloneStorageErrorsAreSafeAndSpecific(t *testing.T) {
	svc := &RemoteService{}
	storageErr := svc.safeCloneStorageError("origin", errCloneStorageQuotaExceeded, maxRemoteClonePackBytes)
	if got, want := storageErr.Error(), `remote "origin" clone exceeded the configured storage byte quota`; got != want {
		t.Fatalf("storage error = %q, want %q", got, want)
	}
	checkoutErr := svc.safeCloneCheckoutError("repo", errCloneEntryQuotaExceeded)
	if got, want := checkoutErr.Error(), `repository "repo" clone exceeded the configured filesystem entry quota during checkout`; got != want {
		t.Fatalf("checkout error = %q, want %q", got, want)
	}
	for _, message := range []string{storageErr.Error(), checkoutErr.Error()} {
		if strings.Contains(message, string(filepath.Separator)+"tmp"+string(filepath.Separator)) || strings.Contains(message, `C:\`) {
			t.Fatalf("safe clone error unexpectedly resembles a filesystem path: %q", message)
		}
	}
}

func TestRemoteCloneCreatesValidatedRepositoryAndPromotesAtomically(t *testing.T) {
	sourceDir := t.TempDir()
	source, err := git.PlainInit(sourceDir, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := source.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	const contents = "guarded clone integration\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "hello.txt"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("hello.txt"); err != nil {
		t.Fatal(err)
	}
	head, err := worktree.Commit("seed clone fixture", &git.CommitOptions{Author: &object.Signature{
		Name: "Clone Test", Email: "clone-test@example.invalid", When: time.Unix(1_700_000_000, 0).UTC(),
	}})
	if err != nil {
		t.Fatal(err)
	}

	objects, err := revlist.Objects(source.Storer, []plumbing.Hash{head}, nil)
	if err != nil {
		t.Fatal(err)
	}
	config, err := source.Storer.Config()
	if err != nil {
		t.Fatal(err)
	}
	var pack bytes.Buffer
	encoder := packfile.NewEncoder(&pack, source.Storer, true)
	if _, err := encoder.Encode(objects, config.Pack.Window); err != nil {
		t.Fatal(err)
	}

	branchRef := plumbing.NewBranchReferenceName("master")
	advertised := packp.NewAdvRefs()
	advertised.References[branchRef.String()] = head
	advertised.Head = &head
	transport := &cloneTestTransport{advertised: advertised, pack: pack.Bytes(), expectedWant: head}

	parent := t.TempDir()
	destination := filepath.Join(parent, "clone-target")
	local := NewServiceWithWriteAccess(map[string]string{"repo": destination}, true)
	remote := newRemoteService(map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://example.com/repo.git", AllowClone: true},
	}, true, false, transport, nil)
	remote.local = local
	remote.cloneEnabled = true
	remote.cloneMaxBytes = 16 << 20
	remote.cloneMaxEntries = 1_000

	result, err := remote.Clone(context.Background(), "origin", "master", head.String())
	if err != nil {
		t.Fatalf("Clone() returned error: %v", err)
	}
	if result.Repository != "repo" || result.Branch != "master" || result.Head != head.String() || result.BytesReceived <= 0 {
		t.Fatalf("unexpected clone result: %#v", result)
	}
	cloned, err := git.PlainOpen(destination)
	if err != nil {
		t.Fatalf("promoted clone could not be opened: %v", err)
	}
	clonedHead, err := cloned.Head()
	if err != nil {
		t.Fatal(err)
	}
	if clonedHead.Name() != branchRef || clonedHead.Hash() != head {
		t.Fatalf("cloned HEAD = %s %s, want %s %s", clonedHead.Name(), clonedHead.Hash(), branchRef, head)
	}
	got, err := os.ReadFile(filepath.Join(destination, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != contents {
		t.Fatalf("cloned file = %q, want %q", got, contents)
	}
	clonedWorktree, err := cloned.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	status, err := clonedWorktree.Status()
	if err != nil || !status.IsClean() {
		t.Fatalf("cloned worktree status = %v, %v; want clean", status, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".omnillm-clone-") {
			t.Fatalf("temporary clone directory remained after promotion: %s", entry.Name())
		}
	}
}

type cloneTestTransport struct {
	advertised   *packp.AdvRefs
	pack         []byte
	expectedWant plumbing.Hash
}

func (t *cloneTestTransport) NewUploadPackSession(*transport.Endpoint, transport.AuthMethod) (transport.UploadPackSession, error) {
	return &cloneTestUploadPackSession{advertised: t.advertised, pack: t.pack, expectedWant: t.expectedWant}, nil
}

func (t *cloneTestTransport) NewReceivePackSession(*transport.Endpoint, transport.AuthMethod) (transport.ReceivePackSession, error) {
	return nil, errors.New("receive-pack is not supported by clone test transport")
}

type cloneTestUploadPackSession struct {
	advertised   *packp.AdvRefs
	pack         []byte
	expectedWant plumbing.Hash
}

func (s *cloneTestUploadPackSession) AdvertisedReferences() (*packp.AdvRefs, error) {
	return s.advertised, nil
}

func (s *cloneTestUploadPackSession) AdvertisedReferencesContext(context.Context) (*packp.AdvRefs, error) {
	return s.advertised, nil
}

func (s *cloneTestUploadPackSession) Close() error { return nil }

func (s *cloneTestUploadPackSession) UploadPack(_ context.Context, request *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	if len(request.Wants) != 1 || request.Wants[0] != s.expectedWant {
		return nil, fmt.Errorf("unexpected clone wants: %#v", request.Wants)
	}
	return packp.NewUploadPackResponseWithPackfile(request, io.NopCloser(bytes.NewReader(s.pack))), nil
}

func clonePackHeader(version, objects uint32) []byte {
	header := make([]byte, 12)
	copy(header[:4], []byte("PACK"))
	binary.BigEndian.PutUint32(header[4:8], version)
	binary.BigEndian.PutUint32(header[8:12], objects)
	return header
}
