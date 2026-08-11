package gitrepo

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestGetPullRequestMergeRequirementsNormalizesRulesAndFailsClosedOnRulesetBypass(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("a", 40)
	requests := 0
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization header = %q", request.Header.Get("Authorization"))
		}
		switch request.URL.Path {
		case "/repos/example/repo/pulls/7":
			return jsonHTTPResponse(http.StatusOK, `{"number":7,"html_url":"https://github.com/example/repo/pull/7","title":"Merge","draft":false,"state":"open","merged":false,"mergeable":true,"mergeable_state":"clean","head":{"ref":"feature/merge","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo":
			return jsonHTTPResponse(http.StatusOK, `{"allow_merge_commit":true,"allow_squash_merge":true,"allow_rebase_merge":true,"permissions":{"admin":true}}`), nil
		case "/repos/example/repo/rules/branches/main":
			if request.URL.Query().Get("per_page") != "100" {
				t.Fatalf("unexpected active rules query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `[
                {"type":"pull_request","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":10,"parameters":{"allowed_merge_methods":["squash","rebase"],"dismiss_stale_reviews_on_push":true,"require_code_owner_review":true,"require_last_push_approval":true,"required_approving_review_count":2,"required_review_thread_resolution":true}},
                {"type":"required_status_checks","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":10,"parameters":{"strict_required_status_checks_policy":true,"required_status_checks":[{"context":"Quality Gate","integration_id":15368}]}},
                {"type":"required_deployments","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":10,"parameters":{"required_deployment_environments":["production"]}},
                {"type":"required_linear_history","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":10},
                {"type":"merge_queue","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":10}
            ]`), nil
		case "/repos/example/repo/branches/main/protection":
			return jsonHTTPResponse(http.StatusOK, `{
                "required_status_checks":{"strict":true,"contexts":["legacy"],"checks":[{"context":"Quality Gate","app_id":15368}]},
                "enforce_admins":{"enabled":false},
                "required_pull_request_reviews":{"dismiss_stale_reviews":true,"require_code_owner_reviews":true,"required_approving_review_count":1,"require_last_push_approval":true},
                "required_linear_history":{"enabled":true},
                "required_conversation_resolution":{"enabled":true}
            }`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestMergeRequirements(context.Background(), "origin", 7)
	if err != nil {
		t.Fatalf("GetPullRequestMergeRequirements() returned error: %v", err)
	}
	if requests != 4 || result.Head != head || result.BaseBranch != "main" || result.Mergeable == nil || !*result.Mergeable {
		t.Fatalf("unexpected result binding: %#v requests=%d", result, requests)
	}
	if result.MergePolicyComplete || result.RulesetBypassVisibility != "incomplete" || !result.PotentialBypass || !result.ConfiguredActorAdmin {
		t.Fatalf("expected fail-closed ruleset bypass state: %#v", result)
	}
	if !result.MergeQueueRequired || !result.StrictStatusChecks || !result.CodeOwnerReviewRequired || !result.LastPushApprovalRequired || !result.ConversationResolutionRequired || !result.LinearHistoryRequired || !result.DismissStaleReviewsOnPush || result.RequiredApprovingReviewCount != 2 {
		t.Fatalf("missing normalized requirements: %#v", result)
	}
	if len(result.AllowedMergeMethods) != 2 || result.AllowedMergeMethods[0] != "rebase" || result.AllowedMergeMethods[1] != "squash" {
		t.Fatalf("allowed methods = %#v", result.AllowedMergeMethods)
	}
	if len(result.RequiredStatusChecks) != 2 || result.RequiredStatusChecks[0].Context != "Quality Gate" || result.RequiredStatusChecks[0].IntegrationID == nil || *result.RequiredStatusChecks[0].IntegrationID != 15368 || result.RequiredStatusChecks[1].Context != "legacy" || result.RequiredStatusChecks[1].IntegrationID != nil {
		t.Fatalf("required status checks = %#v", result.RequiredStatusChecks)
	}
	if len(result.RequiredDeploymentEnvironments) != 1 || result.RequiredDeploymentEnvironments[0] != "production" {
		t.Fatalf("required deployments = %#v", result.RequiredDeploymentEnvironments)
	}
}

func TestGetPullRequestMergeRequirementsCanBeCompleteWithoutRulesets(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("b", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/8":
			return jsonHTTPResponse(http.StatusOK, `{"number":8,"html_url":"https://github.com/example/repo/pull/8","title":"Merge","draft":false,"state":"open","merged":false,"mergeable":true,"mergeable_state":"clean","head":{"ref":"feature/merge","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo":
			return jsonHTTPResponse(http.StatusOK, `{"allow_merge_commit":true,"allow_squash_merge":true,"allow_rebase_merge":false,"permissions":{"admin":false}}`), nil
		case "/repos/example/repo/rules/branches/main":
			return jsonHTTPResponse(http.StatusOK, `[]`), nil
		case "/repos/example/repo/branches/main/protection":
			return jsonHTTPResponse(http.StatusOK, `{"enforce_admins":{"enabled":true},"required_pull_request_reviews":{"required_approving_review_count":1},"required_conversation_resolution":{"enabled":true}}`), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestMergeRequirements(context.Background(), "origin", 8)
	if err != nil {
		t.Fatalf("GetPullRequestMergeRequirements() returned error: %v", err)
	}
	if !result.MergePolicyComplete || result.RulesetBypassVisibility != "not_applicable" || result.ClassicProtectionStatus != "visible" || result.RepositorySettingsStatus != "complete" || result.RequiredApprovingReviewCount != 1 || !result.ConversationResolutionRequired {
		t.Fatalf("unexpected complete policy result: %#v", result)
	}
}

func TestGetPullRequestMergeRequirementsTreatsClassic404AsIncomplete(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("c", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/9":
			return jsonHTTPResponse(http.StatusOK, `{"number":9,"html_url":"https://github.com/example/repo/pull/9","title":"Merge","state":"open","head":{"ref":"feature/merge","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo":
			return jsonHTTPResponse(http.StatusOK, `{"allow_squash_merge":true,"permissions":{"admin":false}}`), nil
		case "/repos/example/repo/rules/branches/main":
			return jsonHTTPResponse(http.StatusOK, `[]`), nil
		case "/repos/example/repo/branches/main/protection":
			return jsonHTTPResponse(http.StatusNotFound, `{"message":"secret-provider-detail"}`), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestMergeRequirements(context.Background(), "origin", 9)
	if err != nil {
		t.Fatalf("GetPullRequestMergeRequirements() returned error: %v", err)
	}
	if result.MergePolicyComplete || result.ClassicProtectionStatus != "unavailable_or_unprotected" {
		t.Fatalf("classic 404 should remain fail-closed: %#v", result)
	}
}

func TestGetPullRequestMergeRequirementsFlagsUnknownMaterialRule(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("d", 40)
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/10":
			return jsonHTTPResponse(http.StatusOK, `{"number":10,"html_url":"https://github.com/example/repo/pull/10","title":"Merge","state":"open","head":{"ref":"feature/merge","sha":"`+head+`"},"base":{"ref":"main"}}`), nil
		case "/repos/example/repo":
			return jsonHTTPResponse(http.StatusOK, `{"allow_squash_merge":true,"permissions":{"admin":false}}`), nil
		case "/repos/example/repo/rules/branches/main":
			return jsonHTTPResponse(http.StatusOK, `[{"type":"required_signatures","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":11}]`), nil
		case "/repos/example/repo/branches/main/protection":
			return jsonHTTPResponse(http.StatusOK, `{"enforce_admins":{"enabled":true}}`), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	result, err := svc.GetPullRequestMergeRequirements(context.Background(), "origin", 10)
	if err != nil {
		t.Fatalf("GetPullRequestMergeRequirements() returned error: %v", err)
	}
	if result.MergePolicyComplete || len(result.UnknownPolicyRules) != 1 || result.UnknownPolicyRules[0] != "required_signatures" {
		t.Fatalf("unknown material rule was not surfaced: %#v", result)
	}
}
