package gitrepo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxGitHubMergeEligibilityReviewRequests = 20
	maxGitHubRequiredDeployments            = 20
	maxGitHubDeploymentsPerEnvironment      = 20
	maxGitHubDeploymentStatusEvidence       = 100
	maxGitHubSignatureEvidenceCommits       = 100
)

// GitHubPullRequestMergeEligibilityReader adds M3A current-state evidence to
// the M2 policy/actor evidence boundary. It remains strictly read-only.
type GitHubPullRequestMergeEligibilityReader interface {
	GetPullRequestMergeEligibility(ctx context.Context, remoteID string, number int) (*GitHubPullRequestMergeEligibilityResult, error)
}

// GitHubPullRequestMergeEligibilityResult reports whether the current hosted PR
// state satisfies every merge prerequisite this implementation can prove. M3A
// never authorizes or performs a merge; DirectMergeSupported is always false.
type GitHubPullRequestMergeEligibilityResult struct {
	Remote                       string                                `json:"remote"`
	Repository                   string                                `json:"repository"`
	PullRequest                  int                                   `json:"pull_request"`
	Head                         string                                `json:"head"`
	BaseBranch                   string                                `json:"base_branch"`
	PolicyEvidenceComplete       bool                                  `json:"policy_evidence_complete"`
	DefaultBaseVerified          bool                                  `json:"default_base_verified"`
	PullRequestStateEligible     bool                                  `json:"pull_request_state_eligible"`
	MergeableKnown               bool                                  `json:"mergeable_known"`
	Mergeable                    bool                                  `json:"mergeable"`
	MergeableState               string                                `json:"mergeable_state,omitempty"`
	StrictBaseCurrent            bool                                  `json:"strict_base_current"`
	RequiredChecksSatisfied      bool                                  `json:"required_checks_satisfied"`
	RequiredChecks               []GitHubRequiredCheckEligibility      `json:"required_checks,omitempty"`
	ReviewDecision               string                                `json:"review_decision,omitempty"`
	ApprovingReviewsObserved     int                                   `json:"approving_reviews_observed"`
	OutstandingCodeOwnerRequests int                                   `json:"outstanding_code_owner_requests"`
	ReviewsSatisfied             bool                                  `json:"reviews_satisfied"`
	ThreadsSatisfied             bool                                  `json:"threads_satisfied"`
	ThreadsInspected             int                                   `json:"threads_inspected"`
	DeploymentsSatisfied         bool                                  `json:"deployments_satisfied"`
	RequiredDeployments          []GitHubRequiredDeploymentEligibility `json:"required_deployments,omitempty"`
	SignaturesSatisfied          bool                                  `json:"signatures_satisfied"`
	EligibilityComplete          bool                                  `json:"eligibility_complete"`
	Eligible                     bool                                  `json:"eligible"`
	DirectMergeSupported         bool                                  `json:"direct_merge_supported"`
	BlockingReasons              []string                              `json:"blocking_reasons,omitempty"`
}

// GitHubRequiredCheckEligibility records bounded state for one normalized M1/M2
// required check context. Provider output and arbitrary URLs are not copied.
type GitHubRequiredCheckEligibility struct {
	Context              string `json:"context"`
	IntegrationID        *int64 `json:"integration_id,omitempty"`
	CheckRunsObserved    int    `json:"check_runs_observed"`
	CommitStatusObserved bool   `json:"commit_status_observed"`
	Satisfied            bool   `json:"satisfied"`
}

// GitHubRequiredDeploymentEligibility records only the required environment and
// latest exact-head deployment state. Deployment IDs and provider prose remain internal.
type GitHubRequiredDeploymentEligibility struct {
	Environment string `json:"environment"`
	State       string `json:"state"`
	Satisfied   bool   `json:"satisfied"`
}

type githubMergeEligibilityGraphQLResponse struct {
	Data struct {
		Repository *struct {
			PullRequest *struct {
				HeadRefOID     string `json:"headRefOid"`
				BaseRefName    string `json:"baseRefName"`
				ReviewDecision string `json:"reviewDecision"`
				ReviewRequests struct {
					TotalCount int `json:"totalCount"`
					Nodes      []struct {
						AsCodeOwner bool `json:"asCodeOwner"`
					} `json:"nodes"`
				} `json:"reviewRequests"`
				LatestOpinionatedReviews struct {
					TotalCount int `json:"totalCount"`
					Nodes      []struct {
						State  string `json:"state"`
						Commit *struct {
							OID string `json:"oid"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"latestOpinionatedReviews"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []githubGraphQLError `json:"errors"`
}

type githubMergeEligibilityCheckRunsResponse struct {
	TotalCount int `json:"total_count"`
	CheckRuns  []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		App        struct {
			ID int64 `json:"id"`
		} `json:"app"`
	} `json:"check_runs"`
}

type githubMergeEligibilityRepositoryResponse struct {
	DefaultBranch string `json:"default_branch"`
}

type githubMergeEligibilityRefResponse struct {
	Ref    string `json:"ref"`
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type githubMergeEligibilityCompareResponse struct {
	Status string `json:"status"`
}

type githubMergeEligibilityDeployment struct {
	ID          int64  `json:"id"`
	SHA         string `json:"sha"`
	Environment string `json:"environment"`
	CreatedAt   string `json:"created_at"`
}

type githubMergeEligibilityDeploymentStatus struct {
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
}

type githubMergeEligibilityCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Verification struct {
			Verified bool `json:"verified"`
		} `json:"verification"`
	} `json:"commit"`
}

const githubMergeEligibilityQuery = `query OmniLLMMergeEligibility($owner: String!, $repository: String!, $number: Int!, $reviewRequestLimit: Int!) {
  repository(owner: $owner, name: $repository) {
    pullRequest(number: $number) {
      headRefOid
      baseRefName
      reviewDecision
      reviewRequests(first: $reviewRequestLimit) {
        totalCount
        nodes { asCodeOwner }
      }
      latestOpinionatedReviews(first: $reviewRequestLimit, writersOnly: true) {
        totalCount
        nodes { state commit { oid } }
      }
    }
  }
}`

// GetPullRequestMergeEligibility performs M3A's bounded read-only current-state
// evaluation. It requires fresh M2 evidence, evaluates exact-head hosted state,
// and re-fetches the PR before returning so no later mutation can rely on stale
// head/base evidence. M3B must still run this method again immediately before any
// future merge request.
func (s *RemoteService) GetPullRequestMergeEligibility(ctx context.Context, remoteID string, number int) (*GitHubPullRequestMergeEligibilityResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	policy, err := s.GetPullRequestMergePolicyEvidence(ctx, remoteID, number)
	if err != nil {
		return nil, err
	}
	remote, owner, repository, token, err := s.githubPullRequestReadConfig(remoteID)
	if err != nil {
		return nil, err
	}

	result := &GitHubPullRequestMergeEligibilityResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, PullRequest: number,
		Head: policy.Head, BaseBranch: policy.BaseBranch, PolicyEvidenceComplete: policy.EvidenceComplete,
		StrictBaseCurrent:       !policy.Requirements.StrictStatusChecks,
		RequiredChecksSatisfied: len(policy.Requirements.RequiredStatusChecks) == 0,
		ReviewsSatisfied:        !gitHubMergeReviewsRequired(&policy.Requirements),
		ThreadsSatisfied:        !policy.Requirements.ConversationResolutionRequired,
		DeploymentsSatisfied:    len(policy.Requirements.RequiredDeploymentEnvironments) == 0,
		SignaturesSatisfied:     !policy.Requirements.RequiredSignatures,
		DirectMergeSupported:    false,
	}
	complete := true
	block := func(reason string) {
		reason = strings.TrimSpace(reason)
		if reason != "" {
			result.BlockingReasons = append(result.BlockingReasons, reason)
		}
	}
	incomplete := func(reason string) {
		complete = false
		block(reason)
	}
	if !policy.EvidenceComplete || !policy.Requirements.MergePolicyComplete {
		incomplete("merge_policy_evidence_incomplete")
		for _, reason := range policy.BlockingReasons {
			block(reason)
		}
	}

	pull, err := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if err != nil {
		return nil, err
	}
	if !validRemoteHash(pull.Head.SHA) || !strings.EqualFold(pull.Head.SHA, policy.Head) || strings.TrimSpace(pull.Base.Ref) != policy.BaseBranch {
		incomplete("pull_request_changed_after_policy_evidence")
		return finalizeGitHubMergeEligibility(result, complete), nil
	}
	result.MergeableState = strings.ToLower(strings.TrimSpace(pull.MergeableState))
	result.PullRequestStateEligible = strings.EqualFold(pull.State, "open") && !pull.Draft && !pull.Merged
	if !result.PullRequestStateEligible {
		block("pull_request_not_open_ready_unmerged")
	}
	if pull.Mergeable == nil {
		incomplete("mergeability_unknown")
	} else {
		result.MergeableKnown = true
		result.Mergeable = *pull.Mergeable
		if !result.Mergeable {
			block("pull_request_not_mergeable")
		}
	}
	if policy.Requirements.MergeQueueRequired {
		block("merge_queue_required")
	}
	if policy.Requirements.BranchLocked {
		block("base_branch_locked")
	}

	defaultVerified, defaultComplete := s.inspectGitHubDefaultBase(ctx, token, owner, repository, policy.BaseBranch)
	result.DefaultBaseVerified = defaultVerified
	if !defaultComplete {
		incomplete("repository_default_branch_unavailable")
	} else if !defaultVerified {
		block("pull_request_base_not_repository_default")
	}

	if policy.Requirements.StrictStatusChecks {
		current, strictComplete := s.inspectGitHubStrictBaseCurrency(ctx, token, owner, repository, policy.BaseBranch, policy.Head)
		result.StrictBaseCurrent = current
		if !strictComplete {
			incomplete("strict_base_currency_unavailable")
		} else if !current {
			block("strict_status_checks_require_updated_base")
		}
	}

	checks, checksSatisfied, checksComplete := s.inspectGitHubRequiredChecks(ctx, token, owner, repository, policy.Head, policy.Requirements.RequiredStatusChecks)
	result.RequiredChecks = checks
	result.RequiredChecksSatisfied = checksSatisfied
	if !checksComplete {
		incomplete("required_status_check_evidence_incomplete")
	} else if !checksSatisfied {
		block("required_status_checks_unsatisfied")
	}

	if gitHubMergeReviewsRequired(&policy.Requirements) {
		reviewDecision, codeOwnerRequests, approvingReviews, reviewsSatisfied, reviewsComplete := s.inspectGitHubReviewEligibility(ctx, token, owner, repository, number, policy.Head, policy.BaseBranch, &policy.Requirements)
		result.ReviewDecision = reviewDecision
		result.ApprovingReviewsObserved = approvingReviews
		result.OutstandingCodeOwnerRequests = codeOwnerRequests
		result.ReviewsSatisfied = reviewsSatisfied
		if !reviewsComplete {
			incomplete("review_evidence_incomplete")
		}
		// GitHub's reviewDecision is an aggregate code-review status, but the
		// documented last-push rule additionally depends on the identity of the
		// actor who made the most recent reviewable push. M3A does not have a
		// bounded provider field that proves that actor relationship, so this
		// requirement remains explicitly incomplete rather than inferred from an
		// APPROVED reviewDecision.
		if policy.Requirements.LastPushApprovalRequired {
			result.ReviewsSatisfied = false
			incomplete("last_push_approval_evidence_unavailable")
		} else if reviewsComplete && !reviewsSatisfied {
			block("required_reviews_unsatisfied")
		}
	}

	if policy.Requirements.ConversationResolutionRequired {
		threads, threadErr := s.GetPullRequestReviewThreads(ctx, remoteID, number, "", maxGitHubReviewThreadLimit)
		if threadErr != nil || threads == nil || !strings.EqualFold(threads.Head, policy.Head) {
			incomplete("review_thread_evidence_incomplete")
		} else {
			result.ThreadsInspected = len(threads.Threads)
			if threads.HasNextPage || threads.TotalCount > maxGitHubReviewThreadLimit {
				incomplete("review_thread_evidence_truncated")
			} else {
				result.ThreadsSatisfied = true
				for _, thread := range threads.Threads {
					if !thread.IsResolved {
						result.ThreadsSatisfied = false
						break
					}
				}
				if !result.ThreadsSatisfied {
					block("required_review_threads_unresolved")
				}
			}
		}
	}

	deployments, deploymentsSatisfied, deploymentsComplete := s.inspectGitHubRequiredDeployments(ctx, token, owner, repository, policy.Head, policy.Requirements.RequiredDeploymentEnvironments)
	result.RequiredDeployments = deployments
	result.DeploymentsSatisfied = deploymentsSatisfied
	if !deploymentsComplete {
		incomplete("required_deployment_evidence_incomplete")
	} else if !deploymentsSatisfied {
		block("required_deployments_unsatisfied")
	}

	if policy.Requirements.RequiredSignatures {
		signaturesSatisfied, signaturesComplete := s.inspectGitHubRequiredSignatures(ctx, token, owner, repository, number)
		result.SignaturesSatisfied = signaturesSatisfied
		if !signaturesComplete {
			incomplete("required_signature_evidence_incomplete")
		} else if !signaturesSatisfied {
			block("required_signatures_unsatisfied")
		}
	}

	latest, latestErr := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if latestErr != nil || !validRemoteHash(latest.Head.SHA) || !strings.EqualFold(latest.Head.SHA, policy.Head) || strings.TrimSpace(latest.Base.Ref) != policy.BaseBranch {
		incomplete("pull_request_changed_during_eligibility_inspection")
	}
	return finalizeGitHubMergeEligibility(result, complete), nil
}

func finalizeGitHubMergeEligibility(result *GitHubPullRequestMergeEligibilityResult, complete bool) *GitHubPullRequestMergeEligibilityResult {
	if result == nil {
		return nil
	}
	result.BlockingReasons = sortedUniqueStrings(result.BlockingReasons)
	result.EligibilityComplete = complete
	result.Eligible = complete && len(result.BlockingReasons) == 0
	result.DirectMergeSupported = false
	return result
}

func gitHubMergeReviewsRequired(requirements *GitHubPullRequestMergeRequirementsResult) bool {
	return requirements != nil && (requirements.RequiredApprovingReviewCount > 0 || requirements.CodeOwnerReviewRequired || requirements.LastPushApprovalRequired)
}

func (s *RemoteService) inspectGitHubDefaultBase(ctx context.Context, token, owner, repository, baseBranch string) (bool, bool) {
	var response githubMergeEligibilityRepositoryResponse
	endpoint := fmt.Sprintf("/repos/%s/%s", owner, repository)
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &response); err != nil {
		return false, false
	}
	defaultBranch, _, err := cleanBranchName(strings.TrimSpace(response.DefaultBranch))
	if err != nil || defaultBranch != strings.TrimSpace(response.DefaultBranch) {
		return false, false
	}
	return defaultBranch == baseBranch, true
}

func (s *RemoteService) inspectGitHubStrictBaseCurrency(ctx context.Context, token, owner, repository, baseBranch, head string) (bool, bool) {
	refEndpoint := fmt.Sprintf("/repos/%s/%s/git/ref/heads/%s", owner, repository, url.PathEscape(baseBranch))
	var ref githubMergeEligibilityRefResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, refEndpoint, nil, http.StatusOK, &ref); err != nil || !validRemoteHash(ref.Object.SHA) {
		return false, false
	}
	compareEndpoint := fmt.Sprintf("/repos/%s/%s/compare/%s...%s", owner, repository, strings.ToLower(ref.Object.SHA), strings.ToLower(head))
	var comparison githubMergeEligibilityCompareResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, compareEndpoint, nil, http.StatusOK, &comparison); err != nil {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(comparison.Status)) {
	case "ahead", "identical":
		return true, true
	case "behind", "diverged":
		return false, true
	default:
		return false, false
	}
}

func (s *RemoteService) inspectGitHubRequiredChecks(ctx context.Context, token, owner, repository, head string, required []GitHubRequiredStatusCheck) ([]GitHubRequiredCheckEligibility, bool, bool) {
	if len(required) == 0 {
		return nil, true, true
	}
	query := url.Values{}
	query.Set("filter", "latest")
	query.Set("per_page", strconv.Itoa(maxGitHubCheckResults+1))
	endpoint := fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?%s", owner, repository, head, query.Encode())
	var checks githubMergeEligibilityCheckRunsResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &checks); err != nil {
		return nil, false, false
	}
	statusQuery := url.Values{}
	statusQuery.Set("per_page", strconv.Itoa(maxGitHubStatusResults+1))
	statusEndpoint := fmt.Sprintf("/repos/%s/%s/commits/%s/status?%s", owner, repository, head, statusQuery.Encode())
	var statuses githubCombinedStatusResponse
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, statusEndpoint, nil, http.StatusOK, &statuses); err != nil {
		return nil, false, false
	}
	if checks.TotalCount > maxGitHubCheckResults || len(checks.CheckRuns) > maxGitHubCheckResults || statuses.TotalCount > maxGitHubStatusResults || len(statuses.Statuses) > maxGitHubStatusResults {
		return nil, false, false
	}

	states := make([]GitHubRequiredCheckEligibility, 0, len(required))
	allSatisfied := true
	complete := true
	for _, requirement := range normalizeGitHubRequiredStatusChecks(required) {
		state := GitHubRequiredCheckEligibility{Context: requirement.Context, IntegrationID: requirement.IntegrationID}
		requireSpecificApp := false
		if requirement.IntegrationID != nil {
			switch {
			case *requirement.IntegrationID == -1:
				// GitHub's sentinel means any app may satisfy this check.
			case *requirement.IntegrationID > 0:
				requireSpecificApp = true
			default:
				complete = false
			}
		}

		checkObserved := false
		checkPassed := true
		for _, check := range checks.CheckRuns {
			if strings.TrimSpace(check.Name) != requirement.Context {
				continue
			}
			if requireSpecificApp && check.App.ID != *requirement.IntegrationID {
				continue
			}
			checkObserved = true
			state.CheckRunsObserved++
			if !gitHubRequiredCheckRunPassed(check.Status, check.Conclusion) {
				checkPassed = false
			}
		}

		statusObserved := false
		statusPassed := true
		for _, status := range statuses.Statuses {
			if strings.TrimSpace(status.Context) != requirement.Context {
				continue
			}
			statusObserved = true
			state.CommitStatusObserved = true
			if !strings.EqualFold(strings.TrimSpace(status.State), "success") {
				statusPassed = false
			}
		}

		if requireSpecificApp {
			state.Satisfied = checkObserved && checkPassed && (!statusObserved || statusPassed)
		} else {
			state.Satisfied = (checkObserved || statusObserved) && (!checkObserved || checkPassed) && (!statusObserved || statusPassed)
		}
		if !state.Satisfied {
			allSatisfied = false
		}
		states = append(states, state)
	}
	return states, allSatisfied, complete
}

func gitHubRequiredCheckRunPassed(status, conclusion string) bool {
	if !strings.EqualFold(strings.TrimSpace(status), "completed") {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(conclusion)) {
	case "success", "neutral", "skipped":
		return true
	default:
		return false
	}
}

func (s *RemoteService) inspectGitHubReviewEligibility(ctx context.Context, token, owner, repository string, number int, head, base string, requirements *GitHubPullRequestMergeRequirementsResult) (string, int, int, bool, bool) {
	variables := map[string]interface{}{
		"owner": owner, "repository": repository, "number": number,
		"reviewRequestLimit": maxGitHubMergeEligibilityReviewRequests,
	}
	var response githubMergeEligibilityGraphQLResponse
	if err := s.doGitHubGraphQL(ctx, token, githubMergeEligibilityQuery, variables, &response); err != nil || len(response.Errors) > 0 || response.Data.Repository == nil || response.Data.Repository.PullRequest == nil {
		return "", 0, 0, false, false
	}
	pull := response.Data.Repository.PullRequest
	if !validRemoteHash(pull.HeadRefOID) || !strings.EqualFold(pull.HeadRefOID, head) || strings.TrimSpace(pull.BaseRefName) != base {
		return "", 0, 0, false, false
	}
	if pull.ReviewRequests.TotalCount < 0 || pull.ReviewRequests.TotalCount < len(pull.ReviewRequests.Nodes) || len(pull.ReviewRequests.Nodes) > maxGitHubMergeEligibilityReviewRequests || pull.ReviewRequests.TotalCount > maxGitHubMergeEligibilityReviewRequests {
		return "", 0, 0, false, false
	}
	latestReviews := pull.LatestOpinionatedReviews
	if latestReviews.TotalCount < 0 || latestReviews.TotalCount < len(latestReviews.Nodes) || len(latestReviews.Nodes) > maxGitHubMergeEligibilityReviewRequests || latestReviews.TotalCount > maxGitHubMergeEligibilityReviewRequests {
		return "", 0, 0, false, false
	}
	codeOwnerRequests := 0
	for _, request := range pull.ReviewRequests.Nodes {
		if request.AsCodeOwner {
			codeOwnerRequests++
		}
	}
	approvingReviews := 0
	for _, review := range latestReviews.Nodes {
		state := strings.ToUpper(strings.TrimSpace(review.State))
		switch state {
		case "APPROVED":
			if requirements != nil && requirements.DismissStaleReviewsOnPush {
				if review.Commit == nil || !validRemoteHash(review.Commit.OID) {
					return "", 0, 0, false, false
				}
				if !strings.EqualFold(review.Commit.OID, head) {
					continue
				}
			}
			approvingReviews++
		case "CHANGES_REQUESTED", "DISMISSED", "COMMENTED", "PENDING":
			// Known non-approving review states do not satisfy the count.
		default:
			return "", 0, 0, false, false
		}
	}
	decision := strings.ToUpper(strings.TrimSpace(pull.ReviewDecision))
	if !gitHubMergeReviewsRequired(requirements) {
		return decision, codeOwnerRequests, approvingReviews, true, true
	}
	if decision != "APPROVED" {
		switch decision {
		case "REVIEW_REQUIRED", "CHANGES_REQUESTED", "":
			return decision, codeOwnerRequests, approvingReviews, false, true
		default:
			return decision, codeOwnerRequests, approvingReviews, false, false
		}
	}
	satisfied := true
	if requirements != nil {
		if requirements.RequiredApprovingReviewCount > approvingReviews {
			satisfied = false
		}
		if requirements.CodeOwnerReviewRequired && codeOwnerRequests > 0 {
			satisfied = false
		}
	}
	return decision, codeOwnerRequests, approvingReviews, satisfied, true
}

func (s *RemoteService) inspectGitHubRequiredDeployments(ctx context.Context, token, owner, repository, head string, environments []string) ([]GitHubRequiredDeploymentEligibility, bool, bool) {
	environments = sortedUniqueStrings(environments)
	if len(environments) == 0 {
		return nil, true, true
	}
	if len(environments) > maxGitHubRequiredDeployments {
		return nil, false, false
	}
	states := make([]GitHubRequiredDeploymentEligibility, 0, len(environments))
	allSatisfied := true
	for _, environment := range environments {
		query := url.Values{}
		query.Set("sha", head)
		query.Set("environment", environment)
		query.Set("per_page", strconv.Itoa(maxGitHubDeploymentsPerEnvironment+1))
		endpoint := fmt.Sprintf("/repos/%s/%s/deployments?%s", owner, repository, query.Encode())
		var deployments []githubMergeEligibilityDeployment
		if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &deployments); err != nil {
			return states, false, false
		}
		state := GitHubRequiredDeploymentEligibility{Environment: environment}
		if len(deployments) == 0 {
			allSatisfied = false
			states = append(states, state)
			continue
		}
		if len(deployments) > maxGitHubDeploymentsPerEnvironment {
			return states, false, false
		}
		deployment, ok := latestGitHubDeploymentForEnvironment(deployments, head, environment)
		if !ok {
			return states, false, false
		}
		statusEndpoint := fmt.Sprintf("/repos/%s/%s/deployments/%d/statuses?per_page=%d", owner, repository, deployment.ID, maxGitHubDeploymentStatusEvidence)
		var statuses []githubMergeEligibilityDeploymentStatus
		if err := s.doGitHubJSON(ctx, token, http.MethodGet, statusEndpoint, nil, http.StatusOK, &statuses); err != nil {
			return states, false, false
		}
		if len(statuses) >= maxGitHubDeploymentStatusEvidence {
			return states, false, false
		}
		if len(statuses) > 0 {
			latestStatus, ok := latestGitHubDeploymentStatus(statuses)
			if !ok {
				return states, false, false
			}
			state.State = strings.ToLower(strings.TrimSpace(latestStatus.State))
			if !knownGitHubDeploymentState(state.State) {
				return states, false, false
			}
		}
		state.Satisfied = state.State == "success"
		if !state.Satisfied {
			allSatisfied = false
		}
		states = append(states, state)
	}
	return states, allSatisfied, true
}

func latestGitHubDeploymentForEnvironment(deployments []githubMergeEligibilityDeployment, head, environment string) (githubMergeEligibilityDeployment, bool) {
	var latest githubMergeEligibilityDeployment
	var latestCreated time.Time
	seen := false
	for _, deployment := range deployments {
		if deployment.ID <= 0 || !validRemoteHash(deployment.SHA) || !strings.EqualFold(deployment.SHA, head) || deployment.Environment != environment {
			return githubMergeEligibilityDeployment{}, false
		}
		created, err := time.Parse(time.RFC3339, strings.TrimSpace(deployment.CreatedAt))
		if err != nil {
			return githubMergeEligibilityDeployment{}, false
		}
		if !seen || created.After(latestCreated) {
			latest = deployment
			latestCreated = created
			seen = true
			continue
		}
		if created.Equal(latestCreated) && deployment.ID != latest.ID {
			return githubMergeEligibilityDeployment{}, false
		}
	}
	return latest, seen
}

func latestGitHubDeploymentStatus(statuses []githubMergeEligibilityDeploymentStatus) (githubMergeEligibilityDeploymentStatus, bool) {
	var latest githubMergeEligibilityDeploymentStatus
	var latestCreated time.Time
	seen := false
	for _, status := range statuses {
		created, err := time.Parse(time.RFC3339, strings.TrimSpace(status.CreatedAt))
		if err != nil {
			return githubMergeEligibilityDeploymentStatus{}, false
		}
		if !seen || created.After(latestCreated) {
			latest = status
			latestCreated = created
			seen = true
			continue
		}
		if created.Equal(latestCreated) && strings.TrimSpace(status.State) != strings.TrimSpace(latest.State) {
			return githubMergeEligibilityDeploymentStatus{}, false
		}
	}
	return latest, seen
}

func knownGitHubDeploymentState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "error", "failure", "inactive", "in_progress", "queued", "pending", "success":
		return true
	default:
		return false
	}
}

func (s *RemoteService) inspectGitHubRequiredSignatures(ctx context.Context, token, owner, repository string, number int) (bool, bool) {
	query := url.Values{}
	query.Set("per_page", strconv.Itoa(maxGitHubSignatureEvidenceCommits))
	endpoint := fmt.Sprintf("/repos/%s/%s/pulls/%d/commits?%s", owner, repository, number, query.Encode())
	var commits []githubMergeEligibilityCommit
	if err := s.doGitHubJSON(ctx, token, http.MethodGet, endpoint, nil, http.StatusOK, &commits); err != nil {
		return false, false
	}
	// Exactly the page limit is ambiguous because another page may exist. M3A
	// intentionally refuses rather than assuming the first page was complete.
	if len(commits) >= maxGitHubSignatureEvidenceCommits {
		return false, false
	}
	for _, commit := range commits {
		if !validRemoteHash(commit.SHA) {
			return false, false
		}
		if !commit.Commit.Verification.Verified {
			return false, true
		}
	}
	return true, true
}
