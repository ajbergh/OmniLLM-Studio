from pathlib import Path

path = Path("backend/internal/gitrepo/github_pull_request_merge_eligibility.go")
text = path.read_text()

text = text.replace(
    '\tReviewDecision               string                                `json:"review_decision,omitempty"`\n\tOutstandingCodeOwnerRequests int                                   `json:"outstanding_code_owner_requests"`',
    '\tReviewDecision               string                                `json:"review_decision,omitempty"`\n\tApprovingReviewsObserved     int                                   `json:"approving_reviews_observed"`\n\tOutstandingCodeOwnerRequests int                                   `json:"outstanding_code_owner_requests"`',
)

text = text.replace(
    '''\t\t\t\tReviewRequests struct {\n\t\t\t\t\tTotalCount int `json:"totalCount"`\n\t\t\t\t\tNodes      []struct {\n\t\t\t\t\t\tAsCodeOwner bool `json:"asCodeOwner"`\n\t\t\t\t\t} `json:"nodes"`\n\t\t\t\t} `json:"reviewRequests"`''',
    '''\t\t\t\tReviewRequests struct {\n\t\t\t\t\tTotalCount int `json:"totalCount"`\n\t\t\t\t\tNodes      []struct {\n\t\t\t\t\t\tAsCodeOwner bool `json:"asCodeOwner"`\n\t\t\t\t\t} `json:"nodes"`\n\t\t\t\t} `json:"reviewRequests"`\n\t\t\t\tLatestOpinionatedReviews struct {\n\t\t\t\t\tTotalCount int `json:"totalCount"`\n\t\t\t\t\tNodes      []struct {\n\t\t\t\t\t\tState  string `json:"state"`\n\t\t\t\t\t\tCommit *struct {\n\t\t\t\t\t\t\tOID string `json:"oid"`\n\t\t\t\t\t\t} `json:"commit"`\n\t\t\t\t\t} `json:"nodes"`\n\t\t\t\t} `json:"latestOpinionatedReviews"`''',
)

text = text.replace(
    '''      reviewRequests(first: $reviewRequestLimit) {\n        totalCount\n        nodes { asCodeOwner }\n      }''',
    '''      reviewRequests(first: $reviewRequestLimit) {\n        totalCount\n        nodes { asCodeOwner }\n      }\n      latestOpinionatedReviews(first: $reviewRequestLimit, writersOnly: true) {\n        totalCount\n        nodes { state commit { oid } }\n      }''',
)

old_call = '''\treviewDecision, codeOwnerRequests, reviewsSatisfied, reviewsComplete := s.inspectGitHubReviewEligibility(ctx, token, owner, repository, number, policy.Head, policy.BaseBranch, &policy.Requirements)\n\tresult.ReviewDecision = reviewDecision\n\tresult.OutstandingCodeOwnerRequests = codeOwnerRequests\n\tresult.ReviewsSatisfied = reviewsSatisfied\n\tif !reviewsComplete {\n\t\tincomplete("review_evidence_incomplete")\n\t}\n\t// GitHub's reviewDecision is an aggregate code-review status, but the\n\t// documented last-push rule additionally depends on the identity of the\n\t// actor who made the most recent reviewable push. M3A does not have a\n\t// bounded provider field that proves that actor relationship, so this\n\t// requirement remains explicitly incomplete rather than inferred from an\n\t// APPROVED reviewDecision.\n\tif policy.Requirements.LastPushApprovalRequired {\n\t\tresult.ReviewsSatisfied = false\n\t\tincomplete("last_push_approval_evidence_unavailable")\n\t} else if reviewsComplete && !reviewsSatisfied {\n\t\tblock("required_reviews_unsatisfied")\n\t}\n'''
new_call = '''\tif gitHubMergeReviewsRequired(&policy.Requirements) {\n\t\treviewDecision, codeOwnerRequests, approvingReviews, reviewsSatisfied, reviewsComplete := s.inspectGitHubReviewEligibility(ctx, token, owner, repository, number, policy.Head, policy.BaseBranch, &policy.Requirements)\n\t\tresult.ReviewDecision = reviewDecision\n\t\tresult.ApprovingReviewsObserved = approvingReviews\n\t\tresult.OutstandingCodeOwnerRequests = codeOwnerRequests\n\t\tresult.ReviewsSatisfied = reviewsSatisfied\n\t\tif !reviewsComplete {\n\t\t\tincomplete("review_evidence_incomplete")\n\t\t}\n\t\t// GitHub's reviewDecision is an aggregate code-review status, but the\n\t\t// documented last-push rule additionally depends on the identity of the\n\t\t// actor who made the most recent reviewable push. M3A does not have a\n\t\t// bounded provider field that proves that actor relationship, so this\n\t\t// requirement remains explicitly incomplete rather than inferred from an\n\t\t// APPROVED reviewDecision.\n\t\tif policy.Requirements.LastPushApprovalRequired {\n\t\t\tresult.ReviewsSatisfied = false\n\t\t\tincomplete("last_push_approval_evidence_unavailable")\n\t\t} else if reviewsComplete && !reviewsSatisfied {\n\t\t\tblock("required_reviews_unsatisfied")\n\t\t}\n\t}\n'''
if old_call not in text:
    raise SystemExit("review call anchor not found")
text = text.replace(old_call, new_call)

start = text.index("func (s *RemoteService) inspectGitHubReviewEligibility(")
end = text.index("\nfunc (s *RemoteService) inspectGitHubRequiredDeployments(", start)
new_func = r'''func (s *RemoteService) inspectGitHubReviewEligibility(ctx context.Context, token, owner, repository string, number int, head, base string, requirements *GitHubPullRequestMergeRequirementsResult) (string, int, int, bool, bool) {
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
'''
text = text[:start] + new_func + text[end:]
path.write_text(text)
