package gitrepo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mergeEligibilityFixture struct {
	checkAppID       int64
	checkConclusion  string
	statusState      string
	reviewDecision   string
	codeOwnerRequest bool
	deploymentState  string
}

func TestGetPullRequestMergeEligibilityCompletesForSatisfiedPolicy(t *testing.T) {
	svc, head := newMergeEligibilityTestService(t, mergeEligibilityFixture{
		checkAppID: 15368, checkConclusion: "success", statusState: "success",
		reviewDecision: "APPROVED", deploymentState: "success",
	})

	result, err := svc.GetPullRequestMergeEligibility(context.Background(), "origin", 21)
	if err != nil {
		t.Fatalf("GetPullRequestMergeEligibility() returned error: %v", err)
	}
	if result.Head != head || !result.PolicyEvidenceComplete || !result.DefaultBaseVerified || !result.PullRequestStateEligible || !result.MergeableKnown || !result.Mergeable {
		t.Fatalf("unexpected state evidence: %#v", result)
	}
	if !result.RequiredChecksSatisfied || !result.ReviewsSatisfied || !result.DeploymentsSatisfied || !result.ThreadsSatisfied || !result.SignaturesSatisfied {
		t.Fatalf("expected satisfied prerequisites: %#v", result)
	}
	if len(result.RequiredChecks) != 1 || result.RequiredChecks[0].IntegrationID == nil || *result.RequiredChecks[0].IntegrationID != 15368 || !result.RequiredChecks[0].Satisfied {
		t.Fatalf("required checks = %#v", result.RequiredChecks)
	}
	if len(result.RequiredDeployments) != 1 || result.RequiredDeployments[0].Environment != "production" || !result.RequiredDeployments[0].Satisfied {
		t.Fatalf("required deployments = %#v", result.RequiredDeployments)
	}
	if result.ReviewDecision != "APPROVED" || result.OutstandingCodeOwnerRequests != 0 {
		t.Fatalf("review evidence = %#v", result)
	}
	if !result.EligibilityComplete || !result.Eligible || len(result.BlockingReasons) != 0 {
		t.Fatalf("expected complete eligible result: %#v", result)
	}
	if result.DirectMergeSupported {
		t.Fatal("M3A must never enable direct merge")
	}
}

func TestGetPullRequestMergeEligibilityRequiresMatchingCheckApp(t *testing.T) {
	svc, _ := newMergeEligibilityTestService(t, mergeEligibilityFixture{
		checkAppID: 999, checkConclusion: "success", statusState: "success",
		reviewDecision: "APPROVED", deploymentState: "success",
	})

	result, err := svc.GetPullRequestMergeEligibility(context.Background(), "origin", 21)
	if err != nil {
		t.Fatalf("GetPullRequestMergeEligibility() returned error: %v", err)
	}
	if !result.EligibilityComplete || result.Eligible || result.RequiredChecksSatisfied {
		t.Fatalf("wrong app must be known-unsatisfied: %#v", result)
	}
	if !containsString(result.BlockingReasons, "required_status_checks_unsatisfied") {
		t.Fatalf("blocking reasons = %#v", result.BlockingReasons)
	}
}

func TestGetPullRequestMergeEligibilityRequiresApprovedReviewDecision(t *testing.T) {
	svc, _ := newMergeEligibilityTestService(t, mergeEligibilityFixture{
		checkAppID: 15368, checkConclusion: "success", statusState: "success",
		reviewDecision: "REVIEW_REQUIRED", codeOwnerRequest: true, deploymentState: "success",
	})

	result, err := svc.GetPullRequestMergeEligibility(context.Background(), "origin", 21)
	if err != nil {
		t.Fatalf("GetPullRequestMergeEligibility() returned error: %v", err)
	}
	if !result.EligibilityComplete || result.Eligible || result.ReviewsSatisfied || result.OutstandingCodeOwnerRequests != 1 {
		t.Fatalf("review requirements must be known-unsatisfied: %#v", result)
	}
	if !containsString(result.BlockingReasons, "required_reviews_unsatisfied") {
		t.Fatalf("blocking reasons = %#v", result.BlockingReasons)
	}
}

func TestGetPullRequestMergeEligibilityRequiresSuccessfulDeployment(t *testing.T) {
	svc, _ := newMergeEligibilityTestService(t, mergeEligibilityFixture{
		checkAppID: 15368, checkConclusion: "success", statusState: "success",
		reviewDecision: "APPROVED", deploymentState: "failure",
	})

	result, err := svc.GetPullRequestMergeEligibility(context.Background(), "origin", 21)
	if err != nil {
		t.Fatalf("GetPullRequestMergeEligibility() returned error: %v", err)
	}
	if !result.EligibilityComplete || result.Eligible || result.DeploymentsSatisfied {
		t.Fatalf("failed deployment must be known-unsatisfied: %#v", result)
	}
	if !containsString(result.BlockingReasons, "required_deployments_unsatisfied") {
		t.Fatalf("blocking reasons = %#v", result.BlockingReasons)
	}
}

func newMergeEligibilityTestService(t *testing.T, fixture mergeEligibilityFixture) (*RemoteService, string) {
	t.Helper()
	svc := newGitHubPullRequestReadTestService()
	head := strings.Repeat("e", 40)
	if fixture.checkConclusion == "" {
		fixture.checkConclusion = "success"
	}
	if fixture.statusState == "" {
		fixture.statusState = "success"
	}
	if fixture.reviewDecision == "" {
		fixture.reviewDecision = "APPROVED"
	}
	if fixture.deploymentState == "" {
		fixture.deploymentState = "success"
	}

	pull := `{"number":21,"html_url":"https://github.com/example/repo/pull/21","title":"M3A","draft":false,"state":"open","merged":false,"mergeable":true,"mergeable_state":"clean","head":{"ref":"feature/m3a","sha":"` + head + `"},"base":{"ref":"main"}}`
	repository := `{"default_branch":"main","allow_merge_commit":true,"allow_squash_merge":true,"allow_rebase_merge":true,"permissions":{"admin":false}}`
	activeRules := `[
		{"type":"pull_request","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":11,"parameters":{"allowed_merge_methods":["squash"],"dismiss_stale_reviews_on_push":true,"require_code_owner_review":true,"require_last_push_approval":true,"required_approving_review_count":1,"required_review_thread_resolution":false,"required_reviewers":[]}},
		{"type":"required_status_checks","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":11,"parameters":{"strict_required_status_checks_policy":false,"required_status_checks":[{"context":"Quality Gate","integration_id":15368}]}},
		{"type":"required_deployments","ruleset_source_type":"Repository","ruleset_source":"example/repo","ruleset_id":11,"parameters":{"required_deployment_environments":["production"]}}
	]`

	svc.githubClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/repos/example/repo/pulls/21":
			return jsonHTTPResponse(http.StatusOK, pull), nil
		case "/repos/example/repo":
			return jsonHTTPResponse(http.StatusOK, repository), nil
		case "/repos/example/repo/rules/branches/main":
			return jsonHTTPResponse(http.StatusOK, activeRules), nil
		case "/repos/example/repo/branches/main/protection":
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		case "/repos/example/repo/rulesets/11":
			return jsonHTTPResponse(http.StatusOK, `{"id":11,"source_type":"Repository","source":"example/repo","enforcement":"active","bypass_actors":[]}`), nil
		case "/repos/example/repo/collaborators/developer/permission":
			return jsonHTTPResponse(http.StatusOK, `{"permission":"write","role_name":"write","user":{"login":"developer","id":42}}`), nil
		case "/repos/example/repo/commits/" + head + "/check-runs":
			if request.URL.Query().Get("filter") != "latest" || request.URL.Query().Get("per_page") != "51" {
				t.Fatalf("unexpected checks query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `{"total_count":1,"check_runs":[{"name":"Quality Gate","status":"completed","conclusion":"`+fixture.checkConclusion+`","app":{"id":`+strconvFormatInt(fixture.checkAppID)+`}}]}`), nil
		case "/repos/example/repo/commits/" + head + "/status":
			return jsonHTTPResponse(http.StatusOK, `{"state":"`+fixture.statusState+`","total_count":0,"statuses":[]}`), nil
		case "/repos/example/repo/deployments":
			if request.URL.Query().Get("sha") != head || request.URL.Query().Get("environment") != "production" || request.URL.Query().Get("per_page") != "1" {
				t.Fatalf("unexpected deployment query: %s", request.URL.RawQuery)
			}
			return jsonHTTPResponse(http.StatusOK, `[{"id":101,"sha":"`+head+`","environment":"production"}]`), nil
		case "/repos/example/repo/deployments/101/statuses":
			return jsonHTTPResponse(http.StatusOK, `[{"state":"`+fixture.deploymentState+`"}]`), nil
		case "/graphql":
			payload, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			var decoded struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatal(err)
			}
			switch decoded.Query {
			case githubMergePolicyEvidenceQuery:
				return jsonHTTPResponse(http.StatusOK, `{"data":{"viewer":{"login":"developer"},"repository":{"ref":{"name":"main","branchProtectionRule":null}}}}`), nil
			case githubMergeEligibilityQuery:
				codeOwnerNodes := `[]`
				totalCount := 0
				if fixture.codeOwnerRequest {
					codeOwnerNodes = `[{"asCodeOwner":true}]`
					totalCount = 1
				}
				return jsonHTTPResponse(http.StatusOK, `{"data":{"repository":{"pullRequest":{"headRefOid":"`+head+`","baseRefName":"main","reviewDecision":"`+fixture.reviewDecision+`","reviewRequests":{"totalCount":`+strconvItoa(totalCount)+`,"nodes":`+codeOwnerNodes+`}}}}}`), nil
			default:
				t.Fatalf("unexpected GraphQL query: %s", decoded.Query)
				return nil, nil
			}
		default:
			t.Fatalf("unexpected GitHub API path: %s", request.URL.Path)
			return nil, nil
		}
	})}
	return svc, head
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

func strconvItoa(value int) string {
	return strconv.Itoa(value)
}
