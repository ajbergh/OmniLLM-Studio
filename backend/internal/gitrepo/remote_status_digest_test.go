package gitrepo

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
)

func TestRemoteStatusReturnsDigestForCompleteBranchNamespace(t *testing.T) {
	advertised := packp.NewAdvRefs()
	for i := 0; i < maxRemoteStatusRefs+5; i++ {
		name := fmt.Sprintf("refs/heads/feature/%03d", i)
		hash := plumbing.NewHash(fmt.Sprintf("%040x", i+1))
		advertised.References[name] = hash
	}
	transport := &cloneTestTransport{advertised: advertised}
	svc := newRemoteService(map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://example.com/repo.git"},
	}, true, false, transport, nil)

	result, err := svc.RemoteStatus(context.Background(), "origin")
	if err != nil {
		t.Fatalf("RemoteStatus() returned error: %v", err)
	}
	if !result.Truncated || len(result.References) != maxRemoteStatusRefs {
		t.Fatalf("RemoteStatus() display = %d refs, truncated=%v; want %d and true", len(result.References), result.Truncated, maxRemoteStatusRefs)
	}
	want := remoteBranchStateDigest(advertised)
	if result.BranchStateDigest != want {
		t.Fatalf("branch_state_digest = %q, want complete namespace digest %q", result.BranchStateDigest, want)
	}

	advertised.References["refs/heads/feature/204"] = plumbing.NewHash("ffffffffffffffffffffffffffffffffffffffff")
	if got := remoteBranchStateDigest(advertised); got == result.BranchStateDigest {
		t.Fatal("change to a branch beyond the displayed 200 refs did not change the complete namespace digest")
	}
}
