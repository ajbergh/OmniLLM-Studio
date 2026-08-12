package gitrepo

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestGetPullRequestMergePolicyEvidenceCompletesForConfirmedUnprotectedStandardActor(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("a", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/7":
			return jsonHTTPResponse(http.StatusOK, `{"number":7,"state":"open","mergeable":true,"mergeable_state":"clean","head":{"ref":"feature/m2","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo":
			return jsonHTTPResponse(http.StatusOK, `{"allow_merge_commit":true,"allow_squash_merge":true,"allow_rebase_merge":true,"permissions":{"admin":false}}`), nil
		case "/repos/example/repo/rules/branches/main":
			return jsonHTTPResponse(http.StatusOK, `[]`), nil
		case "/repos/example/repo/branches/main/protection":
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		case "/graphql":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"viewer":{"login":"developer"},"repository":{"ref":{"name":"main","branchProtectionRule":null}}}}`), nil
		case "/repos/example/repo/collaborators/developer/permission":
			return jsonHTTPResponse(http.StatusOK, `{"permission":"write","role_name":"write","user":{"login":"developer","id":42}}`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestMergePolicyEvidence(context.Background(), "origin", 7)
	if err != nil {
		t.Fatalf("GetPullRequestMergePolicyEvidence() returned error: %v", err)
	}
	if !result.EvidenceComplete || !result.Requirements.MergePolicyComplete {
		t.Fatalf("expected complete evidence: %#v", result)
	}
	if result.ClassicGraphQLStatus != "complete" || result.Requirements.ClassicProtectionStatus != "unprotected_confirmed" || result.Requirements.ClassicPolicyCoverage != "complete" {
		t.Fatalf("classic evidence = %#v", result)
	}
	if result.RulesetDetailStatus != "not_applicable" || result.ConfiguredActorRepositoryRole != "write" || result.ConfiguredActorBypassStatus != "constrained" {
		t.Fatalf("actor/ruleset evidence = %#v", result)
	}
	if result.DirectMergeSupported {
		t.Fatal("M2 must never enable direct merge")
	}
}

func TestGetPullRequestMergePolicyEvidenceFailsClosedWhenRulesetBypassActorsAreHidden(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("b", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/8":
			return jsonHTTPResponse(http.StatusOK, `{"number":8,"state":"open","head":{"ref":"feature/m2","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo":
			return jsonHTTPResponse(http.StatusOK, `{"allow_squash_merge":true,"permissions":{"admin":false}}`), nil
		case "/repos/example/repo/rules/branches/main":
			return jsonHTTPResponse(http.StatusOK, `[{"type":"required_status_checks","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":10,"parameters":{"strict_required_status_checks_policy":false,"required_status_checks":[]}}]`), nil
		case "/repos/example/repo/branches/main/protection":
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		case "/graphql":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"viewer":{"login":"developer"},"repository":{"ref":{"name":"main","branchProtectionRule":null}}}}`), nil
		case "/repos/example/repo/rulesets/10":
			return jsonHTTPResponse(http.StatusOK, `{"id":10,"source_type":"Repository","source":"example/repo","enforcement":"active"}`), nil
		case "/repos/example/repo/collaborators/developer/permission":
			return jsonHTTPResponse(http.StatusOK, `{"permission":"write","role_name":"write","user":{"login":"developer","id":42}}`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestMergePolicyEvidence(context.Background(), "origin", 8)
	if err != nil {
		t.Fatalf("GetPullRequestMergePolicyEvidence() returned error: %v", err)
	}
	if result.EvidenceComplete || result.Requirements.MergePolicyComplete || result.RulesetDetailStatus != "incomplete" {
		t.Fatalf("hidden bypass actors must fail closed: %#v", result)
	}
	if !containsString(result.BlockingReasons, "ruleset_bypass_actors_not_visible") {
		t.Fatalf("blocking reasons = %#v", result.BlockingReasons)
	}
}

func TestGetPullRequestMergePolicyEvidenceSurfacesVisibleRulesetBypassActor(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("c", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/9":
			return jsonHTTPResponse(http.StatusOK, `{"number":9,"state":"open","head":{"ref":"feature/m2","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo":
			return jsonHTTPResponse(http.StatusOK, `{"allow_squash_merge":true,"permissions":{"admin":false}}`), nil
		case "/repos/example/repo/rules/branches/main":
			return jsonHTTPResponse(http.StatusOK, `[{"type":"required_status_checks","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":11,"parameters":{"strict_required_status_checks_policy":false,"required_status_checks":[]}}]`), nil
		case "/repos/example/repo/branches/main/protection":
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		case "/graphql":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"viewer":{"login":"developer"},"repository":{"ref":{"name":"main","branchProtectionRule":null}}}}`), nil
		case "/repos/example/repo/rulesets/11":
			return jsonHTTPResponse(http.StatusOK, `{"id":11,"source_type":"Repository","source":"example/repo","enforcement":"active","bypass_actors":[{"actor_id":123,"actor_type":"Team","bypass_mode":"always"}]}`), nil
		case "/repos/example/repo/collaborators/developer/permission":
			return jsonHTTPResponse(http.StatusOK, `{"permission":"write","role_name":"write","user":{"login":"developer","id":42}}`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestMergePolicyEvidence(context.Background(), "origin", 9)
	if err != nil {
		t.Fatalf("GetPullRequestMergePolicyEvidence() returned error: %v", err)
	}
	if result.EvidenceComplete || !result.RulesetBypassActorsPresent || !result.Requirements.PotentialBypass || result.ConfiguredActorBypassStatus != "potential_bypass" {
		t.Fatalf("ruleset bypass must remain blocking: %#v", result)
	}
}

func TestGetPullRequestMergePolicyEvidenceCorroboratesClassicAndAddsDeployments(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("d", 40)
	protection := `{
		"required_status_checks":{"strict":true,"contexts":[],"checks":[{"context":"Quality Gate","app_id":15368}]},
		"restrictions":{"users":[],"teams":[],"apps":[]},
		"enforce_admins":{"enabled":true},
		"required_pull_request_reviews":{"dismiss_stale_reviews":true,"require_code_owner_reviews":true,"required_approving_review_count":1,"require_last_push_approval":true,"bypass_pull_request_allowances":{"users":[],"teams":[],"apps":[]}},
		"required_signatures":{"enabled":true},
		"required_linear_history":{"enabled":true},
		"required_conversation_resolution":{"enabled":true},
		"lock_branch":{"enabled":false}
	}`
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/10":
			return jsonHTTPResponse(http.StatusOK, `{"number":10,"state":"open","mergeable":true,"mergeable_state":"clean","head":{"ref":"feature/m2","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo":
			return jsonHTTPResponse(http.StatusOK, `{"allow_merge_commit":false,"allow_squash_merge":true,"allow_rebase_merge":false,"permissions":{"admin":false}}`), nil
		case "/repos/example/repo/rules/branches/main":
			return jsonHTTPResponse(http.StatusOK, `[]`), nil
		case "/repos/example/repo/branches/main/protection":
			return jsonHTTPResponse(http.StatusOK, protection), nil
		case "/graphql":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"viewer":{"login":"developer"},"repository":{"ref":{"name":"main","branchProtectionRule":{"dismissesStaleReviews":true,"isAdminEnforced":true,"lockBranch":false,"requiredApprovingReviewCount":1,"requiredDeploymentEnvironments":["production"],"requiresApprovingReviews":true,"requiresCodeOwnerReviews":true,"requiresCommitSignatures":true,"requiresConversationResolution":true,"requiresDeployments":true,"requiresLinearHistory":true,"requiresStatusChecks":true,"requiresStrictStatusChecks":true,"restrictsPushes":false,"requiredStatusChecks":[{"context":"Quality Gate","app":{"databaseId":15368}}],"bypassPullRequestAllowances":{"totalCount":0}}}}}}}`), nil
		case "/repos/example/repo/collaborators/developer/permission":
			return jsonHTTPResponse(http.StatusOK, `{"permission":"write","role_name":"write","user":{"login":"developer","id":42}}`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestMergePolicyEvidence(context.Background(), "origin", 10)
	if err != nil {
		t.Fatalf("GetPullRequestMergePolicyEvidence() returned error: %v", err)
	}
	if !result.EvidenceComplete || result.ClassicGraphQLStatus != "complete" || result.Requirements.ClassicPolicyCoverage != "complete" {
		t.Fatalf("classic corroboration should be complete: %#v", result)
	}
	if !result.Requirements.LastPushApprovalRequired || len(result.Requirements.RequiredDeploymentEnvironments) != 1 || result.Requirements.RequiredDeploymentEnvironments[0] != "production" {
		t.Fatalf("missing REST/GraphQL combined prerequisites: %#v", result.Requirements)
	}
	if len(result.Requirements.RequiredStatusChecks) != 1 || result.Requirements.RequiredStatusChecks[0].IntegrationID == nil || *result.Requirements.RequiredStatusChecks[0].IntegrationID != 15368 {
		t.Fatalf("required status checks = %#v", result.Requirements.RequiredStatusChecks)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
