package gitrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const maxGitHubMergePolicyRulesets = 20

// GitHubPullRequestMergePolicyEvidenceReader adds M2 corroboration to the M1
// normalized merge-requirements reader. It remains strictly read-only.
type GitHubPullRequestMergePolicyEvidenceReader interface {
	GetPullRequestMergePolicyEvidence(ctx context.Context, remoteID string, number int) (*GitHubPullRequestMergePolicyEvidenceResult, error)
}

// GitHubPullRequestMergePolicyEvidenceResult records whether policy sources and
// the configured actor boundary are sufficiently visible for a future M3 merge
// implementation. Direct merge is deliberately not implemented by M2.
type GitHubPullRequestMergePolicyEvidenceResult struct {
	Remote                        string                                   `json:"remote"`
	Repository                    string                                   `json:"repository"`
	PullRequest                   int                                      `json:"pull_request"`
	Head                          string                                   `json:"head"`
	BaseBranch                    string                                   `json:"base_branch"`
	Requirements                  GitHubPullRequestMergeRequirementsResult `json:"requirements"`
	ClassicGraphQLStatus          string                                   `json:"classic_graphql_status"`
	RulesetDetailStatus           string                                   `json:"ruleset_detail_status"`
	RulesetsInspected             int                                      `json:"rulesets_inspected"`
	RulesetBypassActorsPresent    bool                                     `json:"ruleset_bypass_actors_present"`
	ConfiguredActorLogin          string                                   `json:"configured_actor_login,omitempty"`
	ConfiguredActorRepositoryRole string                                   `json:"configured_actor_repository_role,omitempty"`
	ConfiguredActorRoleStatus     string                                   `json:"configured_actor_role_status"`
	ConfiguredActorBypassStatus   string                                   `json:"configured_actor_bypass_status"`
	EvidenceComplete              bool                                     `json:"evidence_complete"`
	DirectMergeSupported          bool                                     `json:"direct_merge_supported"`
	BlockingReasons               []string                                 `json:"blocking_reasons,omitempty"`
}

type githubMergePolicyGraphQLResponse struct {
	Data struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
		Repository *struct {
			Ref *struct {
				Name                 string                             `json:"name"`
				BranchProtectionRule *githubGraphQLBranchProtectionRule `json:"branchProtectionRule"`
			} `json:"ref"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type githubGraphQLBranchProtectionRule struct {
	DismissesStaleReviews          bool     `json:"dismissesStaleReviews"`
	IsAdminEnforced                bool     `json:"isAdminEnforced"`
	LockBranch                     bool     `json:"lockBranch"`
	RequiredApprovingReviewCount   int      `json:"requiredApprovingReviewCount"`
	RequiredDeploymentEnvironments []string `json:"requiredDeploymentEnvironments"`
	RequiresApprovingReviews       bool     `json:"requiresApprovingReviews"`
	RequiresCodeOwnerReviews       bool     `json:"requiresCodeOwnerReviews"`
	RequiresCommitSignatures       bool     `json:"requiresCommitSignatures"`
	RequiresConversationResolution bool     `json:"requiresConversationResolution"`
	RequiresDeployments            bool     `json:"requiresDeployments"`
	RequiresLinearHistory          bool     `json:"requiresLinearHistory"`
	RequiresStatusChecks           bool     `json:"requiresStatusChecks"`
	RequiresStrictStatusChecks     bool     `json:"requiresStrictStatusChecks"`
	RestrictsPushes                bool     `json:"restrictsPushes"`
	BypassPullRequestAllowances    struct {
		TotalCount int `json:"totalCount"`
	} `json:"bypassPullRequestAllowances"`
	RequiredStatusChecks []struct {
		Context string `json:"context"`
		App     *struct {
			DatabaseID *int64 `json:"databaseId"`
		} `json:"app"`
	} `json:"requiredStatusChecks"`
}

type githubMergeRulesetDetail struct {
	ID           int64           `json:"id"`
	SourceType   string          `json:"source_type"`
	Source       string          `json:"source"`
	Enforcement  string          `json:"enforcement"`
	BypassActors json.RawMessage `json:"bypass_actors"`
}

type githubMergeRulesetBypassActor struct {
	ActorID    *int64 `json:"actor_id"`
	ActorType  string `json:"actor_type"`
	BypassMode string `json:"bypass_mode"`
}

type githubViewerRepositoryPermission struct {
	Permission string `json:"permission"`
	RoleName   string `json:"role_name"`
	User       struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
}

const githubMergePolicyEvidenceQuery = `query OmniLLMMergePolicyEvidence($owner: String!, $repository: String!, $qualifiedRef: String!) {
  viewer { login }
  repository(owner: $owner, name: $repository) {
    ref(qualifiedName: $qualifiedRef) {
      name
      branchProtectionRule {
        dismissesStaleReviews
        isAdminEnforced
        lockBranch
        requiredApprovingReviewCount
        requiredDeploymentEnvironments
        requiresApprovingReviews
        requiresCodeOwnerReviews
        requiresCommitSignatures
        requiresConversationResolution
        requiresDeployments
        requiresLinearHistory
        requiresStatusChecks
        requiresStrictStatusChecks
        restrictsPushes
        requiredStatusChecks { context app { databaseId } }
        bypassPullRequestAllowances(first: 1) { totalCount }
      }
    }
  }
}`

// GetPullRequestMergePolicyEvidence performs M2's bounded read-only evidence
// pass. It starts from a fresh M1 result, corroborates classic protection via
// REST plus exact-ref GraphQL, inspects every bounded active ruleset detail for
// bypass visibility, verifies the configured actor's repository role, then
// re-fetches the PR to ensure evidence remained bound to the same head/base.
func (s *RemoteService) GetPullRequestMergePolicyEvidence(ctx context.Context, remoteID string, number int) (*GitHubPullRequestMergePolicyEvidenceResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	base, err := s.GetPullRequestMergeRequirements(ctx, remoteID, number)
	if err != nil {
		return nil, err
	}
	remote, owner, repository, token, err := s.githubPullRequestReadConfig(remoteID)
	if err != nil {
		return nil, err
	}

	result := &GitHubPullRequestMergePolicyEvidenceResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, PullRequest: number,
		Head: base.Head, BaseBranch: base.BaseBranch, Requirements: cloneGitHubMergeRequirements(base),
		ClassicGraphQLStatus: "unavailable", RulesetDetailStatus: "unavailable",
		ConfiguredActorRoleStatus: "unavailable", ConfiguredActorBypassStatus: "unproven",
		DirectMergeSupported: false, BlockingReasons: []string{},
	}

	viewerLogin, classicStatus, classicReasons := s.inspectGitHubClassicPolicyEvidence(ctx, token, owner, repository, base.BaseBranch, &result.Requirements)
	result.ConfiguredActorLogin = viewerLogin
	result.ClassicGraphQLStatus = classicStatus
	result.BlockingReasons = append(result.BlockingReasons, classicReasons...)

	rulesetStatus, rulesetsInspected, bypassActorsPresent, rulesetReasons := s.inspectGitHubRulesetBypassEvidence(ctx, token, owner, repository, base.BaseBranch)
	result.RulesetDetailStatus = rulesetStatus
	result.RulesetsInspected = rulesetsInspected
	result.RulesetBypassActorsPresent = bypassActorsPresent
	result.BlockingReasons = append(result.BlockingReasons, rulesetReasons...)
	switch rulesetStatus {
	case "not_applicable":
		result.Requirements.RulesetBypassVisibility = "not_applicable"
	case "complete":
		result.Requirements.RulesetBypassVisibility = "complete"
	default:
		result.Requirements.RulesetBypassVisibility = "incomplete"
	}

	if result.ConfiguredActorLogin != "" {
		permissionEndpoint := fmt.Sprintf("/repos/%s/%s/collaborators/%s/permission", owner, repository, url.PathEscape(result.ConfiguredActorLogin))
		var permission githubViewerRepositoryPermission
		permissionStatus, permissionErr := s.doGitHubReadJSONStatus(ctx, token, permissionEndpoint, &permission)
		if permissionErr == nil && permissionStatus == http.StatusOK && strings.EqualFold(permission.User.Login, result.ConfiguredActorLogin) {
			roleName := strings.ToLower(strings.TrimSpace(permission.RoleName))
			result.ConfiguredActorRepositoryRole = roleName
			if standardGitHubRepositoryRole(roleName) {
				result.ConfiguredActorRoleStatus = "standard"
				actorAdmin := roleName == "admin"
				if result.Requirements.ConfiguredActorAdmin != actorAdmin {
					result.ConfiguredActorRoleStatus = "inconsistent"
					result.BlockingReasons = append(result.BlockingReasons, "configured_actor_permission_sources_inconsistent")
				}
			} else if roleName != "" {
				result.ConfiguredActorRoleStatus = "custom_role_unverified"
				result.BlockingReasons = append(result.BlockingReasons, "configured_actor_custom_role_permissions_unverified")
			} else {
				result.BlockingReasons = append(result.BlockingReasons, "configured_actor_repository_role_unavailable")
			}
		} else {
			result.BlockingReasons = append(result.BlockingReasons, "configured_actor_repository_role_unavailable")
		}
	} else {
		result.BlockingReasons = append(result.BlockingReasons, "configured_actor_identity_unavailable")
	}

	// M1 conservatively marks any active ruleset as a possible bypass because
	// the active-rules endpoint omits bypass actors. M2 may clear only that
	// placeholder after every ruleset detail exposed its bypass_actors field.
	if classicStatus == "complete" && (rulesetStatus == "complete" || rulesetStatus == "not_applicable") {
		result.Requirements.PotentialBypass = bypassActorsPresent || result.Requirements.ClassicReviewBypassAllowancesPresent
		if result.Requirements.ConfiguredActorAdmin && result.Requirements.ClassicProtectionStatus == "visible" {
			if result.Requirements.ClassicAdministratorEnforced == nil || !*result.Requirements.ClassicAdministratorEnforced {
				result.Requirements.PotentialBypass = true
			}
		}
	}

	result.ConfiguredActorBypassStatus = configuredGitHubActorBypassStatus(&result.Requirements, result.ConfiguredActorRoleStatus, result.ConfiguredActorRepositoryRole, bypassActorsPresent)
	if result.ConfiguredActorBypassStatus != "constrained" {
		result.BlockingReasons = append(result.BlockingReasons, "configured_actor_bypass_not_proven_absent")
	}

	result.Requirements.RequiredStatusChecks = normalizeGitHubRequiredStatusChecks(result.Requirements.RequiredStatusChecks)
	result.Requirements.RequiredDeploymentEnvironments = sortedUniqueStrings(result.Requirements.RequiredDeploymentEnvironments)
	result.Requirements.UnknownPolicyRules = sortedUniqueStrings(result.Requirements.UnknownPolicyRules)
	result.Requirements.AllowedMergeMethods = sortedUniqueStrings(result.Requirements.AllowedMergeMethods)
	result.BlockingReasons = sortedUniqueStrings(result.BlockingReasons)

	result.Requirements.MergePolicyComplete = result.Requirements.RepositorySettingsStatus == "complete" &&
		result.Requirements.ActiveRulesStatus == "complete" && !result.Requirements.ActiveRulesTruncated &&
		result.Requirements.ClassicPolicyCoverage == "complete" &&
		(result.Requirements.RulesetBypassVisibility == "complete" || result.Requirements.RulesetBypassVisibility == "not_applicable") &&
		len(result.Requirements.UnknownPolicyRules) == 0 && !result.Requirements.PotentialBypass
	result.EvidenceComplete = classicStatus == "complete" &&
		(rulesetStatus == "complete" || rulesetStatus == "not_applicable") &&
		result.ConfiguredActorRoleStatus == "standard" && result.ConfiguredActorBypassStatus == "constrained" &&
		result.Requirements.MergePolicyComplete && len(result.BlockingReasons) == 0

	latest, latestErr := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if latestErr != nil || !strings.EqualFold(strings.TrimSpace(latest.Head.SHA), base.Head) || strings.TrimSpace(latest.Base.Ref) != base.BaseBranch {
		result.EvidenceComplete = false
		result.Requirements.MergePolicyComplete = false
		result.BlockingReasons = sortedUniqueStrings(append(result.BlockingReasons, "pull_request_changed_during_policy_inspection"))
	}

	// M2 is an evidence gate only. Complete evidence never registers or
	// authorizes a merge mutation; M3 requires an independent implementation.
	result.DirectMergeSupported = false
	return result, nil
}

func cloneGitHubMergeRequirements(input *GitHubPullRequestMergeRequirementsResult) GitHubPullRequestMergeRequirementsResult {
	if input == nil {
		return GitHubPullRequestMergeRequirementsResult{}
	}
	out := *input
	out.AllowedMergeMethods = append([]string(nil), input.AllowedMergeMethods...)
	out.RequiredStatusChecks = append([]GitHubRequiredStatusCheck(nil), input.RequiredStatusChecks...)
	out.RequiredDeploymentEnvironments = append([]string(nil), input.RequiredDeploymentEnvironments...)
	out.UnknownPolicyRules = append([]string(nil), input.UnknownPolicyRules...)
	return out
}

func (s *RemoteService) inspectGitHubClassicPolicyEvidence(ctx context.Context, token, owner, repository, baseBranch string, result *GitHubPullRequestMergeRequirementsResult) (string, string, []string) {
	var graph githubMergePolicyGraphQLResponse
	graphErr := s.doGitHubGraphQL(ctx, token, githubMergePolicyEvidenceQuery, map[string]interface{}{
		"owner": owner, "repository": repository, "qualifiedRef": "refs/heads/" + baseBranch,
	}, &graph)
	if graphErr != nil || len(graph.Errors) > 0 || graph.Data.Repository == nil || graph.Data.Repository.Ref == nil || strings.TrimSpace(graph.Data.Repository.Ref.Name) != baseBranch {
		return "", "unavailable", []string{"classic_graphql_visibility_incomplete"}
	}
	viewerLogin := strings.TrimSpace(graph.Data.Viewer.Login)
	rule := graph.Data.Repository.Ref.BranchProtectionRule

	protectionEndpoint := fmt.Sprintf("/repos/%s/%s/branches/%s/protection", owner, repository, url.PathEscape(baseBranch))
	var protection githubBranchProtectionResponse
	protectionStatus, protectionErr := s.doGitHubReadJSONStatus(ctx, token, protectionEndpoint, &protection)
	if protectionErr != nil {
		return viewerLogin, "unavailable", []string{"classic_rest_visibility_incomplete"}
	}

	if rule == nil {
		if protectionStatus == http.StatusOK {
			return viewerLogin, "inconsistent", []string{"classic_policy_sources_inconsistent"}
		}
		result.ClassicProtectionStatus = "unprotected_confirmed"
		result.ClassicPolicyCoverage = "complete"
		result.ClassicAdministratorEnforced = nil
		result.ClassicRestrictionsPresent = false
		result.ClassicReviewBypassAllowancesPresent = false
		result.UnknownPolicyRules = removeGitHubPolicyMarkers(result.UnknownPolicyRules,
			"classic.restrictions", "classic.bypass_pull_request_allowances")
		return viewerLogin, "complete", nil
	}

	// GraphQL exposes deployment prerequisites and integration-bound status
	// checks, but REST still carries material classic fields such as last-push
	// approval. A visible GraphQL rule therefore cannot replace inaccessible
	// REST branch protection; the two sources must corroborate each other.
	if protectionStatus != http.StatusOK {
		return viewerLogin, "incomplete", []string{"classic_rest_visibility_incomplete"}
	}
	if !githubClassicRESTGraphQLConsistent(protection, rule) {
		return viewerLogin, "inconsistent", []string{"classic_policy_sources_inconsistent"}
	}

	applyGitHubClassicProtection(result, protection)
	applyGitHubGraphQLClassicEvidence(result, rule)
	result.ClassicProtectionStatus = "visible"
	result.ClassicPolicyCoverage = "complete"
	return viewerLogin, "complete", nil
}

func applyGitHubGraphQLClassicEvidence(result *GitHubPullRequestMergeRequirementsResult, rule *githubGraphQLBranchProtectionRule) {
	if result == nil || rule == nil {
		return
	}
	adminEnforced := rule.IsAdminEnforced
	result.ClassicAdministratorEnforced = &adminEnforced
	result.DismissStaleReviewsOnPush = result.DismissStaleReviewsOnPush || rule.DismissesStaleReviews
	if rule.RequiresApprovingReviews && rule.RequiredApprovingReviewCount > result.RequiredApprovingReviewCount {
		result.RequiredApprovingReviewCount = rule.RequiredApprovingReviewCount
	}
	result.CodeOwnerReviewRequired = result.CodeOwnerReviewRequired || rule.RequiresCodeOwnerReviews
	result.RequiredSignatures = result.RequiredSignatures || rule.RequiresCommitSignatures
	result.ConversationResolutionRequired = result.ConversationResolutionRequired || rule.RequiresConversationResolution
	result.LinearHistoryRequired = result.LinearHistoryRequired || rule.RequiresLinearHistory
	result.BranchLocked = result.BranchLocked || rule.LockBranch
	if rule.RequiresStatusChecks {
		result.StrictStatusChecks = result.StrictStatusChecks || rule.RequiresStrictStatusChecks
		result.RequiredStatusChecks = append(result.RequiredStatusChecks, graphQLRequiredStatusChecks(rule)...)
	}
	if rule.RequiresDeployments {
		if len(rule.RequiredDeploymentEnvironments) == 0 {
			result.UnknownPolicyRules = append(result.UnknownPolicyRules, "classic.required_deployments.environments")
		} else {
			result.RequiredDeploymentEnvironments = append(result.RequiredDeploymentEnvironments, rule.RequiredDeploymentEnvironments...)
		}
	}
	if rule.RestrictsPushes {
		result.ClassicRestrictionsPresent = true
		result.UnknownPolicyRules = append(result.UnknownPolicyRules, "classic.restrictions")
	} else {
		result.ClassicRestrictionsPresent = false
		result.UnknownPolicyRules = removeGitHubPolicyMarkers(result.UnknownPolicyRules, "classic.restrictions")
	}
	if rule.BypassPullRequestAllowances.TotalCount > 0 {
		result.ClassicReviewBypassAllowancesPresent = true
		result.PotentialBypass = true
		result.UnknownPolicyRules = append(result.UnknownPolicyRules, "classic.bypass_pull_request_allowances")
	} else {
		result.ClassicReviewBypassAllowancesPresent = false
		result.UnknownPolicyRules = removeGitHubPolicyMarkers(result.UnknownPolicyRules, "classic.bypass_pull_request_allowances")
	}
}

func githubClassicRESTGraphQLConsistent(protection githubBranchProtectionResponse, rule *githubGraphQLBranchProtectionRule) bool {
	if rule == nil {
		return false
	}
	if protection.EnforceAdmins != nil && protection.EnforceAdmins.Enabled != rule.IsAdminEnforced {
		return false
	}
	if protection.RequiredPullRequestReviews == nil {
		if rule.RequiresApprovingReviews || rule.RequiredApprovingReviewCount > 0 || rule.RequiresCodeOwnerReviews || rule.DismissesStaleReviews || rule.BypassPullRequestAllowances.TotalCount > 0 {
			return false
		}
	} else {
		reviews := protection.RequiredPullRequestReviews
		if !rule.RequiresApprovingReviews || reviews.RequiredApprovingReviewCount != rule.RequiredApprovingReviewCount || reviews.RequireCodeOwnerReviews != rule.RequiresCodeOwnerReviews || reviews.DismissStaleReviews != rule.DismissesStaleReviews {
			return false
		}
		count, ok := githubClassicRESTBypassActorCount(reviews.BypassPullRequestAllowances)
		if !ok || count != rule.BypassPullRequestAllowances.TotalCount {
			return false
		}
	}
	if (protection.RequiredSignatures != nil && protection.RequiredSignatures.Enabled) != rule.RequiresCommitSignatures {
		return false
	}
	if (protection.RequiredLinearHistory != nil && protection.RequiredLinearHistory.Enabled) != rule.RequiresLinearHistory {
		return false
	}
	if (protection.RequiredConversationResolution != nil && protection.RequiredConversationResolution.Enabled) != rule.RequiresConversationResolution {
		return false
	}
	if (protection.LockBranch != nil && protection.LockBranch.Enabled) != rule.LockBranch {
		return false
	}
	restRestrictions, ok := githubClassicRESTRestrictionsPresent(protection.Restrictions)
	if !ok || restRestrictions != rule.RestrictsPushes {
		return false
	}
	if protection.RequiredStatusChecks == nil {
		return !rule.RequiresStatusChecks && len(rule.RequiredStatusChecks) == 0
	}
	if !rule.RequiresStatusChecks || protection.RequiredStatusChecks.Strict != rule.RequiresStrictStatusChecks {
		return false
	}
	return equalGitHubStatusChecks(classicRESTRequiredStatusChecks(protection.RequiredStatusChecks), graphQLRequiredStatusChecks(rule))
}

func classicRESTRequiredStatusChecks(checks *struct {
	Strict   bool     `json:"strict"`
	Contexts []string `json:"contexts"`
	Checks   []struct {
		Context string `json:"context"`
		AppID   *int64 `json:"app_id"`
	} `json:"checks"`
}) []GitHubRequiredStatusCheck {
	if checks == nil {
		return nil
	}
	out := make([]GitHubRequiredStatusCheck, 0, len(checks.Checks)+len(checks.Contexts))
	if len(checks.Checks) > 0 {
		for _, check := range checks.Checks {
			out = append(out, GitHubRequiredStatusCheck{Context: check.Context, IntegrationID: check.AppID})
		}
		return normalizeGitHubRequiredStatusChecks(out)
	}
	for _, contextName := range checks.Contexts {
		out = append(out, GitHubRequiredStatusCheck{Context: contextName})
	}
	return normalizeGitHubRequiredStatusChecks(out)
}

func graphQLRequiredStatusChecks(rule *githubGraphQLBranchProtectionRule) []GitHubRequiredStatusCheck {
	if rule == nil {
		return nil
	}
	out := make([]GitHubRequiredStatusCheck, 0, len(rule.RequiredStatusChecks))
	for _, check := range rule.RequiredStatusChecks {
		contextName := strings.TrimSpace(check.Context)
		if contextName == "" {
			continue
		}
		var integrationID *int64
		if check.App != nil && check.App.DatabaseID != nil {
			value := *check.App.DatabaseID
			integrationID = &value
		}
		out = append(out, GitHubRequiredStatusCheck{Context: contextName, IntegrationID: integrationID})
	}
	return normalizeGitHubRequiredStatusChecks(out)
}

func equalGitHubStatusChecks(left, right []GitHubRequiredStatusCheck) bool {
	left = normalizeGitHubRequiredStatusChecks(left)
	right = normalizeGitHubRequiredStatusChecks(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Context != right[i].Context {
			return false
		}
		var leftID, rightID int64
		if left[i].IntegrationID != nil {
			leftID = *left[i].IntegrationID
		}
		if right[i].IntegrationID != nil {
			rightID = *right[i].IntegrationID
		}
		if leftID != rightID {
			return false
		}
	}
	return true
}

func githubClassicRESTBypassActorCount(raw json.RawMessage) (int, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return 0, true
	}
	var value struct {
		Users []json.RawMessage `json:"users"`
		Teams []json.RawMessage `json:"teams"`
		Apps  []json.RawMessage `json:"apps"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return 0, false
	}
	return len(value.Users) + len(value.Teams) + len(value.Apps), true
}

func githubClassicRESTRestrictionsPresent(raw json.RawMessage) (bool, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return false, true
	}
	var value struct {
		Users []json.RawMessage `json:"users"`
		Teams []json.RawMessage `json:"teams"`
		Apps  []json.RawMessage `json:"apps"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return false, false
	}
	return len(value.Users)+len(value.Teams)+len(value.Apps) > 0, true
}

func removeGitHubPolicyMarkers(values []string, remove ...string) []string {
	removeSet := map[string]bool{}
	for _, value := range remove {
		removeSet[value] = true
	}
	out := values[:0]
	for _, value := range values {
		if !removeSet[value] {
			out = append(out, value)
		}
	}
	return out
}

func (s *RemoteService) inspectGitHubRulesetBypassEvidence(ctx context.Context, token, owner, repository, baseBranch string) (string, int, bool, []string) {
	query := url.Values{}
	query.Set("per_page", strconv.Itoa(maxGitHubMergePolicyRules))
	endpoint := fmt.Sprintf("/repos/%s/%s/rules/branches/%s?%s", owner, repository, url.PathEscape(baseBranch), query.Encode())
	var rules []githubActiveBranchRule
	status, err := s.doGitHubReadJSONStatus(ctx, token, endpoint, &rules)
	if err != nil || status != http.StatusOK {
		return "incomplete", 0, false, []string{"active_rules_unavailable_for_ruleset_evidence"}
	}
	if len(rules) >= maxGitHubMergePolicyRules {
		return "incomplete", 0, false, []string{"active_rules_truncated_for_ruleset_evidence"}
	}
	if len(rules) == 0 {
		return "not_applicable", 0, false, nil
	}

	type sourceInfo struct{ sourceType, source string }
	rulesets := map[int64]sourceInfo{}
	for _, rule := range rules {
		if rule.RulesetID <= 0 {
			return "incomplete", 0, false, []string{"active_rule_missing_ruleset_id"}
		}
		info := sourceInfo{sourceType: strings.TrimSpace(rule.RulesetSourceType), source: strings.TrimSpace(rule.RulesetSource)}
		if existing, ok := rulesets[rule.RulesetID]; ok && existing != info {
			return "incomplete", 0, false, []string{"active_ruleset_source_inconsistent"}
		}
		rulesets[rule.RulesetID] = info
	}
	if len(rulesets) > maxGitHubMergePolicyRulesets {
		return "incomplete", 0, false, []string{"active_ruleset_detail_bound_exceeded"}
	}

	ids := make([]int64, 0, len(rulesets))
	for id := range rulesets {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	bypassPresent := false
	for index, id := range ids {
		detailEndpoint := fmt.Sprintf("/repos/%s/%s/rulesets/%d?includes_parents=true", owner, repository, id)
		var detail githubMergeRulesetDetail
		detailStatus, detailErr := s.doGitHubReadJSONStatus(ctx, token, detailEndpoint, &detail)
		if detailErr != nil || detailStatus != http.StatusOK || detail.ID != id {
			return "incomplete", index, bypassPresent, []string{"ruleset_detail_unavailable"}
		}
		expected := rulesets[id]
		if !strings.EqualFold(strings.TrimSpace(detail.SourceType), expected.sourceType) || strings.TrimSpace(detail.Source) != expected.source {
			return "incomplete", index + 1, bypassPresent, []string{"ruleset_detail_source_inconsistent"}
		}
		if strings.ToLower(strings.TrimSpace(detail.Enforcement)) != "active" {
			return "incomplete", index + 1, bypassPresent, []string{"ruleset_detail_enforcement_unexpected"}
		}
		raw := strings.TrimSpace(string(detail.BypassActors))
		if raw == "" || raw == "null" {
			return "incomplete", index + 1, bypassPresent, []string{"ruleset_bypass_actors_not_visible"}
		}
		var actors []githubMergeRulesetBypassActor
		if json.Unmarshal(detail.BypassActors, &actors) != nil {
			return "incomplete", index + 1, bypassPresent, []string{"ruleset_bypass_actors_invalid"}
		}
		if len(actors) > 0 {
			bypassPresent = true
		}
	}
	return "complete", len(ids), bypassPresent, nil
}

func standardGitHubRepositoryRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "read", "triage", "write", "maintain", "admin":
		return true
	default:
		return false
	}
}

func configuredGitHubActorBypassStatus(requirements *GitHubPullRequestMergeRequirementsResult, roleStatus, role string, rulesetBypassActorsPresent bool) string {
	if requirements == nil || roleStatus != "standard" {
		return "unproven"
	}
	if rulesetBypassActorsPresent || requirements.ClassicReviewBypassAllowancesPresent {
		return "potential_bypass"
	}
	if strings.EqualFold(role, "admin") && requirements.ClassicProtectionStatus == "visible" {
		if requirements.ClassicAdministratorEnforced == nil || !*requirements.ClassicAdministratorEnforced {
			return "potential_bypass"
		}
	}
	if requirements.PotentialBypass {
		return "potential_bypass"
	}
	return "constrained"
}
