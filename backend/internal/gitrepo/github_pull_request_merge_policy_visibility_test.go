package gitrepo

import (
	"context"
	"net/http"
	"testing"
)

func TestInspectGitHubClassicPolicyEvidenceDoesNotTreatForbiddenProtectionAsUnprotected(t *testing.T) {
	svc := newGitHubPullRequestReadTestService()
	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/graphql":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"viewer":{"login":"developer"},"repository":{"ref":{"name":"main","branchProtectionRule":null}}}}`), nil
		case "/repos/example/repo/branches/main/protection":
			return jsonHTTPResponse(http.StatusForbidden, `{"message":"Resource not accessible"}`), nil
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	requirements := GitHubPullRequestMergeRequirementsResult{
		ClassicProtectionStatus: "unavailable_or_unprotected",
		ClassicPolicyCoverage:   "rest_partial",
	}
	login, status, reasons := svc.inspectGitHubClassicPolicyEvidence(context.Background(), "token", "example", "repo", "main", &requirements)
	if login != "developer" {
		t.Fatalf("viewer login = %q, want developer", login)
	}
	if status != "incomplete" {
		t.Fatalf("classic evidence status = %q, want incomplete", status)
	}
	if requirements.ClassicProtectionStatus == "unprotected_confirmed" || requirements.ClassicPolicyCoverage == "complete" {
		t.Fatalf("forbidden REST visibility must remain fail closed: %#v", requirements)
	}
	if !containsString(reasons, "classic_rest_visibility_incomplete") {
		t.Fatalf("blocking reasons = %#v", reasons)
	}
}
