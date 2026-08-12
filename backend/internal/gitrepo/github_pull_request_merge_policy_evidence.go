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

// GitHubPullRequestMergePolicyEvidenceResult records whether the policy sources
// and configured actor boundary are sufficiently visible for a future M3 merge
// implementation. Direct merge is not implemented by M2.
type GitHubPullRequestMergePolicyEvidenceResult struct {
	Remote                         string                                     `json:"remote"`
	Repository                     string                                     `json:"repository"`
	PullRequest                    int                                        `json:"pull_request"`
	Head                           string                                     `json:"head"`
	BaseBranch                     string                                     `json:"base_branch"`
	Requirements                   GitHubPullRequestMergeRequirementsResult    `json:"requirements"`
	ClassicGraphQLStatus           string                                     `json:"classic_graphql_status"`
	RulesetDetailStatus            string                                     `json:"ruleset_detail_status"`
	RulesetsInspected              int                                        `json:"rulesets_inspected"`
	RulesetBypassActorsPresent     bool                                       `json:"ruleset_bypass_actors_present"`
	ConfiguredActorLogin           string                                     `json:"configured_actor_login,omitempty"`
	ConfiguredActorRepositoryRole  string                                     `json:"configured_actor_repository_role,omitempty"`
	ConfiguredActorRoleStatus      string                                     `json:"configured_actor_role_status"`
	ConfiguredActorBypassStatus    string                                     `json:"configured_actor_bypass_status"`
	EvidenceComplete               bool                                       `json:"evidence_complete"`
	DirectMergeSupported           bool                                       `json:"direct_merge_supported"`
	BlockingReasons                []string                                   `json:"blocking_reasons,omitempty"`
}

type githubMergePolicyGraphQLResponse struct {
	Viewer struct {
		Login string `json:"login"`
	} `json:"viewer"`
	Repository *struct {
		Ref *struct {
			Name                 string                             `json:"name"`
			BranchProtectionRule *githubGraphQLBranchProtectionRule `json:"branchProtectionRule"`
		} `json:"ref"`
	} `json:"repository"`
}

type githubGraphQLBranchProtectionRule struct {
	DismissesStaleReviews          bool `json:"dismissesStaleReviews"`
	IsAdminEnforced                bool `json:"isAdminEnforced"`
	LockBranch                     bool `json:"lockBranch"`
	RequiredApprovingReviewCount   int  `json:"requiredApprovingReviewCount"`
	RequiredDeploymentEnvironments []string `json:"requiredDeploymentEnvironments"`
	RequiresApprovingReviews       bool `json:"requiresApprovingReviews"`
	RequiresCodeOwnerReviews       bool `json:"requiresCodeOwnerReviews"`
	RequiresCommitSignatures       bool `json:"requiresCommitSignatures"`
	RequiresConversationResolution bool `json:"requiresConversationResolution"`
	RequiresDeployments            bool `json:"requiresDeployments"`
	RequiresLinearHistory          bool `json:"requiresLinearHistory"`
	RequiresStatusChecks           bool `json:"requiresStatusChecks"`
	RequiresStrictStatusChecks     bool `json:"requiresStrictStatusChecks"`
	RestrictsPushes                bool `json:"restrictsPushes"`
	BypassPullRequestAllowances    struct {
		TotalCount int `json:"totalCount"`
	} `json:"bypassPullRequestAllowances"`
	PushAllowances struct {
		TotalCount int `json:"totalCount"`
	} `json:"pushAllowances"`
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
        pushAllowances(first: 1) { totalCount }
      }
    }
  }
}`

// GetPullRequestMergePolicyEvidence performs M2's bounded read-only evidence
// pass. It starts from a fresh M1 result, corroborates classic protection via
// exact-ref GraphQL, inspects each bounded active ruleset detail for bypass
// visibility, verifies the configured actor's repository role, then re-fetches
// the PR to ensure the evidence remained bound to the same head/base.
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

	effective := cloneGitHubMergeRequirements(base)
	result := &GitHubPullRequestMergePolicyEvidenceResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, PullRequest: number,
		Head: base.Head, BaseBranch: base.BaseBranch, Requirements: effective,
		ClassicGraphQLStatus: "unavailable", RulesetDetailStatus: "unavailable",
		ConfiguredActorRoleStatus: "unavailable", ConfiguredActorBypassStatus: "unproven",
		DirectMergeSupported: false, BlockingReasons: []string{},
	}

	var graph githubMergePolicyGraphQLResponse
	graphErr := s.doGitHubGraphQL(ctx, token, githubMergePolicyEvidenceQuery, map[string]interface{}{
		"owner": owner, "repository": repository, "qualifiedRef": "refs/heads/" + base.BaseBranch,
	}, &graph)
	if graphErr != nil || graph.Repository == nil || graph.Repository.Ref == nil || strings.TrimSpace(graph.Repository.Ref.Name) != base.BaseBranch {
		result.BlockingReasons = append(result.BlockingReasons, "classic_graphql_visibility_incomplete")
	} else {
		result.ConfiguredActorLogin = strings.TrimSpace(graph.Viewer.Login)
		if err := applyGitHubGraphQLClassicEvidence(&result.Requirements, graph.Repository.Ref.BranchProtectionRule); err != nil {
			result.ClassicGraphQLStatus = "inconsistent"
			result.BlockingReasons = append(result.BlockingReasons, "classic_policy_sources_inconsistent")
		} else {
			result.ClassicGraphQLStatus = "complete"
		}
	}

	rulesetStatus, rulesetsInspected, bypassActorsPresent, rulesetReasons := s.inspectGitHubRulesetBypassEvidence(ctx, token, owner, repository, base.BaseBranch)
	result.RulesetDetailStatus = rulesetStatus
	result.RulesetsInspected = rulesetsInspected
	result.RulesetBypassActorsPresent = bypassActorsPresent
	result.BlockingReasons = append(result.BlockingReasons, rulesetReasons...)
	if rulesetStatus == "not_applicable" {
		result.Requirements.RulesetBypassVisibility = "not_applicable"
	} else if rulesetStatus == "complete" {
		result.Requirements.RulesetBypassVisibility = "complete"
	} else {
		result.Requirements.RulesetBypassVisibility = "incomplete"
	}
	if bypassActorsPresent {
		result.Requirements.PotentialBypass = true
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
			} else if roleName != "" {
				result.ConfiguredActorRoleStatus = "custom_role_unverified"
				result.BlockingReasons = append(result.BlockingReasons, "configured_actor_custom_role_permissions_unverified")
			} else {
				result.ConfiguredActorRoleStatus = "unavailable"
				result.BlockingReasons = append(result.BlockingReasons, "configured_actor_repository_role_unavailable")
			}
		} else {
			result.BlockingReasons = append(result.BlockingReasons, "configured_actor_repository_role_unavailable")
		}
	} else {
		result.BlockingReasons = append(result.BlockingReasons, "configured_actor_identity_unavailable")
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

	result.EvidenceComplete = result.ClassicGraphQLStatus == "complete" &&
		(result.RulesetDetailStatus == "complete" || result.RulesetDetailStatus == "not_applicable") &&
		result.ConfiguredActorRoleStatus == "standard" && result.ConfiguredActorBypassStatus == "constrained" &&
		result.Requirements.MergePolicyComplete && len(result.BlockingReasons) == 0

	latest, latestErr := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if latestErr != nil || !strings.EqualFold(strings.TrimSpace(latest.Head.SHA), base.Head) || strings.TrimSpace(latest.Base.Ref) != base.BaseBranch {
		result.EvidenceComplete = false
		result.Requirements.MergePolicyComplete = false
		result.BlockingReasons = sortedUniqueStrings(append(result.BlockingReasons, "pull_request_changed_during_policy_inspection"))
	}

	// M2 is an evidence gate only. Even complete evidence does not register or
	// authorize a merge mutation; M3 requires an independent implementation.
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

func applyGitHubGraphQLClassicEvidence(result *GitHubPullRequestMergeRequirementsResult, rule *githubGraphQLBranchProtectionRule) error {
	if result == nil {
		return fmt.Errorf("merge requirements are unavailable")
	}
	if rule == nil {
		if result.ClassicProtectionStatus == "visible" {
			return fmt.Errorf("classic protection sources disagree")
		}
		result.ClassicProtectionStatus = "unprotected_confirmed"
		result.ClassicPolicyCoverage = "complete"
		result.ClassicAdministratorEnforced = nil
		result.ClassicRestrictionsPresent = false
		result.ClassicReviewBypassAllowancesPresent = false
		return nil
	}

	result.ClassicProtectionStatus = "visible"
	result.ClassicPolicyCoverage = "complete"
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
		for _, check := range rule.RequiredStatusChecks {
			contextName := strings.TrimSpace(check.Context)
			if contextName == "" {
				result.UnknownPolicyRules = append(result.UnknownPolicyRules, "classic.required_status_checks.context")
				continue
			}
			var integrationID *int64
			if check.App != nil && check.App.DatabaseID != nil {
				value := *check.App.DatabaseID
				integrationID = &value
			}
			result.RequiredStatusChecks = append(result.RequiredStatusChecks, GitHubRequiredStatusCheck{Context: contextName, IntegrationID: integrationID})
		}
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
	}
	if rule.BypassPullRequestAllowances.TotalCount > 0 {
		result.ClassicReviewBypassAllowancesPresent = true
		result.PotentialBypass = true
		result.UnknownPolicyRules = append(result.UnknownPolicyRules, "classic.bypass_pull_request_allowances")
	}
	if result.ConfiguredActorAdmin && !rule.IsAdminEnforced {
		result.PotentialBypass = true
	}
	return nil
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
		if existing, ok := rulesets[rule.RulesetID]; ok && (existing != info) {
			return "incomplete", 0, false, []string{"active_ruleset_source_inconsistent"}
		}
		rulesets[rule.RulesetID] = info
	}
	if len(rulesets) > maxGitHubMergePolicyRulesets {
		return "incomplete", 0, false, []string{"active_ruleset_detail_bound_exceeded"}
	}

	ids := make([]int64, 0, len(rulesets))
	for id := range rulesets { ids = append(ids, id) }
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
