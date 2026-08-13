package tools

import (
	"context"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/gitrepo"
)

func TestConfigureGitHubCredentialsBootstrapsBindingOnlyRemoteInspection(t *testing.T) {
	t.Setenv(gitrepo.RepositoriesEnv, "omni="+t.TempDir())
	t.Setenv(gitrepo.RemotesEnv, "")
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")

	options := &GitHubCredentialOptions{
		Connected: func(_ context.Context, userID string) (bool, error) { return userID == "user-binding", nil },
		Resolve: func(_ context.Context, userID string) (string, bool, error) {
			if userID != "user-binding" {
				return "", false, nil
			}
			return "user-token", true, nil
		},
		Bindings: func(_ context.Context, userID string) ([]gitrepo.GitHubRemoteBinding, error) {
			if userID != "user-binding" {
				return nil, nil
			}
			return []gitrepo.GitHubRemoteBinding{{LocalRepositoryID: "omni", GitHubFullName: "octo/studio"}}, nil
		},
	}

	registry := NewRegistryWithOptions(RegistryOptions{GitHubCredentials: options})
	tool, ok := registry.Get("git_remotes")
	if !ok {
		t.Fatal("binding-only configuration should register git_remotes")
	}
	if _, ok := registry.Get("git_remote_status"); !ok {
		t.Fatal("binding-only configuration should register git_remote_status")
	}
	remoteTool, ok := tool.(*gitRemoteTool)
	if !ok {
		t.Fatalf("unexpected git_remotes tool type %T", tool)
	}
	scoped, ok := remoteTool.service.(*gitrepo.UserScopedRemoteService)
	if !ok {
		t.Fatalf("binding-only remote service was not request-scoped: %T", remoteTool.service)
	}
	ctx := ContextWithInvocationScope(context.Background(), InvocationScope{UserID: "user-binding"})
	remotes := scoped.Remotes(ctx)
	if len(remotes) != 1 {
		t.Fatalf("expected one binding-backed remote, got %#v", remotes)
	}
	if got, want := remotes[0].ID, gitrepo.GitHubBindingRemoteID("omni"); got != want {
		t.Fatalf("binding remote ID = %q, want %q", got, want)
	}
	if !remotes[0].AuthenticationConfigured {
		t.Fatal("connected binding-backed remote should report authentication configured")
	}
	if remotes[0].PushAllowed || remotes[0].PullRequestReadAllowed || remotes[0].PullRequestCreateAllowed || remotes[0].PullRequestReadyAllowed || remotes[0].PullRequestMergeAllowed {
		t.Fatalf("binding-only bootstrap widened remote permissions: %#v", remotes[0])
	}
}

func TestBindingOnlyBootstrapDoesNotRegisterMutationOrHostedGitHubTools(t *testing.T) {
	t.Setenv(gitrepo.RepositoriesEnv, "omni="+t.TempDir())
	t.Setenv(gitrepo.RemotesEnv, "")
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")

	registry := NewRegistryWithOptions(RegistryOptions{GitHubCredentials: &GitHubCredentialOptions{
		Connected: func(context.Context, string) (bool, error) { return true, nil },
		Resolve:   func(context.Context, string) (string, bool, error) { return "token", true, nil },
		Bindings: func(context.Context, string) ([]gitrepo.GitHubRemoteBinding, error) {
			return []gitrepo.GitHubRemoteBinding{{LocalRepositoryID: "omni", GitHubFullName: "octo/studio"}}, nil
		},
	}})

	for _, name := range []string{
		"git_fetch", "git_push", "git_publish_branch", "git_clone",
		"github_get_pull_request", "github_create_draft_pull_request",
		"github_reply_to_pull_request_review_comment", "github_set_pull_request_review_thread_resolved",
		"github_mark_pull_request_ready_for_review", "github_get_pull_request_merge_requirements",
		"github_get_pull_request_merge_policy_evidence", "github_get_pull_request_merge_eligibility",
		"github_merge_pull_request",
	} {
		if _, ok := registry.Get(name); ok {
			t.Fatalf("binding-only bootstrap must not register %s", name)
		}
	}
}

func TestBindingOnlyBootstrapRequiresBindingCallbacksAndRemoteReadGate(t *testing.T) {
	t.Setenv(gitrepo.RepositoriesEnv, "omni="+t.TempDir())
	t.Setenv(gitrepo.RemotesEnv, "")
	t.Setenv(gitrepo.RemoteEnabledEnv, "true")

	withoutBindings := NewRegistryWithOptions(RegistryOptions{GitHubCredentials: &GitHubCredentialOptions{
		Connected: func(context.Context, string) (bool, error) { return true, nil },
		Resolve:   func(context.Context, string) (string, bool, error) { return "token", true, nil },
	}})
	if _, ok := withoutBindings.Get("git_remotes"); ok {
		t.Fatal("credential callbacks without bindings must preserve no-static-remote behavior")
	}

	t.Setenv(gitrepo.RemoteEnabledEnv, "false")
	remoteDisabled := NewRegistryWithOptions(RegistryOptions{GitHubCredentials: &GitHubCredentialOptions{
		Connected: func(context.Context, string) (bool, error) { return true, nil },
		Resolve:   func(context.Context, string) (string, bool, error) { return "token", true, nil },
		Bindings:  func(context.Context, string) ([]gitrepo.GitHubRemoteBinding, error) { return nil, nil },
	}})
	if _, ok := remoteDisabled.Get("git_remotes"); ok {
		t.Fatal("binding callbacks must not bypass the process-wide remote read gate")
	}
}
