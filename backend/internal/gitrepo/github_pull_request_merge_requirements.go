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

const maxGitHubMergePolicyRules = 100

// GitHubPullRequestMergeRequirementsReader exposes bounded, read-only merge
// policy inspection for one pull request. Repository identity, API host,
// credentials, base branch, and head SHA remain operator/GitHub derived.
type GitHubPullRequestMergeRequirementsReader interface {
	GetPullRequestMergeRequirements(ctx context.Context, remoteID string, number int) (*GitHubPullRequestMergeRequirementsResult, error)
}

// GitHubRequiredStatusCheck is one normalized required status/check context.
// IntegrationID is included only when GitHub binds the requirement to one app.
type GitHubRequiredStatusCheck struct {
	Context       string `json:"context"`
	IntegrationID *int64 `json:"integration_id,omitempty"`
}

// GitHubPullRequestMergeRequirementsResult is a fail-closed normalized view of
// active merge policy for the current PR head/base. MergePolicyComplete is true
// only when every policy source used by this implementation was visible and no
// unsupported material rule or bypass ambiguity remains.
type GitHubPullRequestMergeRequirementsResult struct {
	Remote                               string                      `json:"remote"`
	Repository                           string                      `json:"repository"`
	PullRequest                          int                         `json:"pull_request"`
	Head                                 string                      `json:"head"`
	BaseBranch                           string                      `json:"base_branch"`
	State                                string                      `json:"state"`
	Draft                                bool                        `json:"draft"`
	Merged                               bool                        `json:"merged"`
	Mergeable                            *bool                       `json:"mergeable"`
	MergeableState                       string                      `json:"mergeable_state,omitempty"`
	MergePolicyComplete                  bool                        `json:"merge_policy_complete"`
	ActiveRulesStatus                    string                      `json:"active_rules_status"`
	ActiveRulesTruncated                 bool                        `json:"active_rules_truncated,omitempty"`
	ClassicProtectionStatus              string                      `json:"classic_protection_status"`
	ClassicPolicyCoverage                string                      `json:"classic_policy_coverage"`
	RepositorySettingsStatus             string                      `json:"repository_settings_status"`
	RulesetBypassVisibility              string                      `json:"ruleset_bypass_visibility"`
	ConfiguredActorAdmin                 bool                        `json:"configured_actor_admin"`
	ClassicAdministratorEnforced         *bool                       `json:"classic_administrator_enforced,omitempty"`
	ClassicRestrictionsPresent           bool                        `json:"classic_restrictions_present"`
	ClassicReviewBypassAllowancesPresent bool                        `json:"classic_review_bypass_allowances_present"`
	PotentialBypass                      bool                        `json:"potential_bypass"`
	MergeQueueRequired                   bool                        `json:"merge_queue_required"`
	AllowedMergeMethods                  []string                    `json:"allowed_merge_methods"`
	RequiredStatusChecks                 []GitHubRequiredStatusCheck `json:"required_status_checks"`
	StrictStatusChecks                   bool                        `json:"strict_status_checks"`
	RequiredApprovingReviewCount         int                         `json:"required_approving_review_count"`
	CodeOwnerReviewRequired              bool                        `json:"code_owner_review_required"`
	LastPushApprovalRequired             bool                        `json:"last_push_approval_required"`
	DismissStaleReviewsOnPush            bool                        `json:"dismiss_stale_reviews_on_push"`
	ConversationResolutionRequired       bool                        `json:"conversation_resolution_required"`
	RequiredDeploymentEnvironments       []string                    `json:"required_deployment_environments"`
	LinearHistoryRequired                bool                        `json:"linear_history_required"`
	RequiredSignatures                   bool                        `json:"required_signatures"`
	BranchLocked                         bool                        `json:"branch_locked"`
	UnknownPolicyRules                   []string                    `json:"unknown_policy_rules,omitempty"`
}

type githubActiveBranchRule struct {
	Type              string          `json:"type"`
	RulesetSourceType string          `json:"ruleset_source_type"`
	RulesetSource     string          `json:"ruleset_source"`
	RulesetID         int64           `json:"ruleset_id"`
	Parameters        json.RawMessage `json:"parameters"`
}

type githubPullRequestRuleParameters struct {
	AllowedMergeMethods            []string          `json:"allowed_merge_methods"`
	DismissStaleReviewsOnPush      bool              `json:"dismiss_stale_reviews_on_push"`
	RequireCodeOwnerReview         bool              `json:"require_code_owner_review"`
	RequireLastPushApproval        bool              `json:"require_last_push_approval"`
	RequiredApprovingReviewCount   int               `json:"required_approving_review_count"`
	RequiredReviewThreadResolution bool              `json:"required_review_thread_resolution"`
	RequiredReviewers              []json.RawMessage `json:"required_reviewers"`
}

type githubRequiredStatusChecksRuleParameters struct {
	StrictRequiredStatusChecksPolicy bool `json:"strict_required_status_checks_policy"`
	RequiredStatusChecks             []struct {
		Context       string `json:"context"`
		IntegrationID *int64 `json:"integration_id"`
	} `json:"required_status_checks"`
}

type githubRequiredDeploymentsRuleParameters struct {
	RequiredDeploymentEnvironments []string `json:"required_deployment_environments"`
}

type githubBranchProtectionResponse struct {
	RequiredStatusChecks *struct {
		Strict   bool     `json:"strict"`
		Contexts []string `json:"contexts"`
		Checks   []struct {
			Context string `json:"context"`
			AppID   *int64 `json:"app_id"`
		} `json:"checks"`
	} `json:"required_status_checks"`
	Restrictions  json.RawMessage `json:"restrictions"`
	EnforceAdmins *struct {
		Enabled bool `json:"enabled"`
	} `json:"enforce_admins"`
	RequiredPullRequestReviews *struct {
		DismissStaleReviews          bool            `json:"dismiss_stale_reviews"`
		RequireCodeOwnerReviews      bool            `json:"require_code_owner_reviews"`
		RequiredApprovingReviewCount int             `json:"required_approving_review_count"`
		RequireLastPushApproval      bool            `json:"require_last_push_approval"`
		BypassPullRequestAllowances  json.RawMessage `json:"bypass_pull_request_allowances"`
	} `json:"required_pull_request_reviews"`
	RequiredSignatures *struct {
		Enabled bool `json:"enabled"`
	} `json:"required_signatures"`
	RequiredLinearHistory *struct {
		Enabled bool `json:"enabled"`
	} `json:"required_linear_history"`
	RequiredConversationResolution *struct {
		Enabled bool `json:"enabled"`
	} `json:"required_conversation_resolution"`
	LockBranch *struct {
		Enabled bool `json:"enabled"`
	} `json:"lock_branch"`
}

type githubRepositoryMergeSettingsResponse struct {
	AllowMergeCommit bool `json:"allow_merge_commit"`
	AllowSquashMerge bool `json:"allow_squash_merge"`
	AllowRebaseMerge bool `json:"allow_rebase_merge"`
	Permissions      struct {
		Admin bool `json:"admin"`
	} `json:"permissions"`
}

// GetPullRequestMergeRequirements reads normalized merge requirements for the
// exact current PR head and base. Ambiguous or inaccessible policy sources are
// represented as incomplete policy rather than guessed as unprotected.
func (s *RemoteService) GetPullRequestMergeRequirements(ctx context.Context, remoteID string, number int) (*GitHubPullRequestMergeRequirementsResult, error) {
	if number <= 0 {
		return nil, fmt.Errorf("pull request number must be positive")
	}
	remote, owner, repository, token, err := s.githubPullRequestReadConfig(remoteID)
	if err != nil {
		return nil, err
	}
	pull, err := s.getGitHubPullRequest(ctx, token, owner, repository, number)
	if err != nil {
		return nil, err
	}
	if !validRemoteHash(pull.Head.SHA) {
		return nil, fmt.Errorf("GitHub pull request head could not be validated")
	}
	baseBranch, _, err := cleanBranchName(strings.TrimSpace(pull.Base.Ref))
	if err != nil || baseBranch != strings.TrimSpace(pull.Base.Ref) {
		return nil, fmt.Errorf("GitHub pull request base branch could not be validated")
	}

	result := &GitHubPullRequestMergeRequirementsResult{
		Remote: strings.TrimSpace(remoteID), Repository: remote.Repository, PullRequest: number,
		Head: strings.ToLower(pull.Head.SHA), BaseBranch: baseBranch, State: pull.State, Draft: pull.Draft, Merged: pull.Merged,
		Mergeable: pull.Mergeable, MergeableState: pull.MergeableState,
		ActiveRulesStatus: "unavailable", ClassicProtectionStatus: "unavailable_or_unprotected", ClassicPolicyCoverage: "unavailable",
		RepositorySettingsStatus: "unavailable", RulesetBypassVisibility: "unknown",
		AllowedMergeMethods: []string{}, RequiredStatusChecks: []GitHubRequiredStatusCheck{},
		RequiredDeploymentEnvironments: []string{}, UnknownPolicyRules: []string{},
	}

	repositoryEndpoint := fmt.Sprintf("/repos/%s/%s", owner, repository)
	var repositorySettings githubRepositoryMergeSettingsResponse
	repositoryStatus, repositoryErr := s.doGitHubReadJSONStatus(ctx, token, repositoryEndpoint, &repositorySettings)
	if repositoryErr != nil {
		return nil, fmt.Errorf("GitHub repository merge settings could not be inspected")
	}
	if repositoryStatus == http.StatusOK {
		result.RepositorySettingsStatus = "complete"
		result.ConfiguredActorAdmin = repositorySettings.Permissions.Admin
		result.AllowedMergeMethods = repositoryMergeMethods(repositorySettings)
	}

	rulesQuery := url.Values{}
	rulesQuery.Set("per_page", strconv.Itoa(maxGitHubMergePolicyRules))
	rulesEndpoint := fmt.Sprintf("/repos/%s/%s/rules/branches/%s?%s", owner, repository, url.PathEscape(baseBranch), rulesQuery.Encode())
	var activeRules []githubActiveBranchRule
	rulesStatus, rulesErr := s.doGitHubReadJSONStatus(ctx, token, rulesEndpoint, &activeRules)
	if rulesErr != nil {
		return nil, fmt.Errorf("GitHub active branch rules could not be inspected")
	}
	if rulesStatus == http.StatusOK {
		result.ActiveRulesStatus = "complete"
		if len(activeRules) >= maxGitHubMergePolicyRules {
			result.ActiveRulesTruncated = true
		}
		if len(activeRules) == 0 {
			result.RulesetBypassVisibility = "not_applicable"
		} else {
			// The active-rules endpoint intentionally omits ruleset bypass actors.
			// Until M2 adds a separately validated actor/bypass read, policy remains
			// incomplete rather than assuming the configured identity is constrained.
			result.RulesetBypassVisibility = "incomplete"
			result.PotentialBypass = true
		}
		applyGitHubActiveBranchRules(result, activeRules)
	}

	protectionEndpoint := fmt.Sprintf("/repos/%s/%s/branches/%s/protection", owner, repository, url.PathEscape(baseBranch))
	var protection githubBranchProtectionResponse
	protectionStatus, protectionErr := s.doGitHubReadJSONStatus(ctx, token, protectionEndpoint, &protection)
	if protectionErr != nil {
		return nil, fmt.Errorf("GitHub classic branch protection could not be inspected")
	}
	if protectionStatus == http.StatusOK {
		result.ClassicProtectionStatus = "visible"
		// The REST branch-protection response does not expose every classic
		// merge prerequisite (notably required deployments). Until a fixed
		// GraphQL BranchProtectionRule read corroborates those fields, REST-only
		// coverage must never be treated as complete merge policy.
		result.ClassicPolicyCoverage = "rest_partial"
		applyGitHubClassicProtection(result, protection)
		if protection.EnforceAdmins != nil {
			enforced := protection.EnforceAdmins.Enabled
			result.ClassicAdministratorEnforced = &enforced
			if result.ConfiguredActorAdmin && !enforced {
				result.PotentialBypass = true
			}
		}
	}

	if result.LinearHistoryRequired {
		result.AllowedMergeMethods = removeString(result.AllowedMergeMethods, "merge")
	}
	result.RequiredStatusChecks = normalizeGitHubRequiredStatusChecks(result.RequiredStatusChecks)
	result.RequiredDeploymentEnvironments = sortedUniqueStrings(result.RequiredDeploymentEnvironments)
	result.UnknownPolicyRules = sortedUniqueStrings(result.UnknownPolicyRules)
	result.AllowedMergeMethods = sortedUniqueStrings(result.AllowedMergeMethods)

	result.MergePolicyComplete = result.RepositorySettingsStatus == "complete" &&
		result.ActiveRulesStatus == "complete" && !result.ActiveRulesTruncated &&
		result.ClassicProtectionStatus == "visible" && result.ClassicPolicyCoverage == "complete" &&
		(result.RulesetBypassVisibility == "not_applicable") &&
		len(result.UnknownPolicyRules) == 0
	if result.ConfiguredActorAdmin && result.ClassicAdministratorEnforced != nil && !*result.ClassicAdministratorEnforced {
		result.MergePolicyComplete = false
	}
	return result, nil
}

func applyGitHubActiveBranchRules(result *GitHubPullRequestMergeRequirementsResult, rules []githubActiveBranchRule) {
	if result == nil {
		return
	}
	for _, rule := range rules {
		ruleType := strings.TrimSpace(rule.Type)
		switch ruleType {
		case "pull_request":
			var parameters githubPullRequestRuleParameters
			if len(rule.Parameters) == 0 || json.Unmarshal(rule.Parameters, &parameters) != nil {
				result.UnknownPolicyRules = append(result.UnknownPolicyRules, "pull_request.parameters")
				continue
			}
			if len(parameters.AllowedMergeMethods) == 0 {
				result.UnknownPolicyRules = append(result.UnknownPolicyRules, "pull_request.allowed_merge_methods")
			} else if len(result.AllowedMergeMethods) > 0 {
				result.AllowedMergeMethods = intersectMergeMethods(result.AllowedMergeMethods, parameters.AllowedMergeMethods)
			}
			if parameters.RequiredApprovingReviewCount > result.RequiredApprovingReviewCount {
				result.RequiredApprovingReviewCount = parameters.RequiredApprovingReviewCount
			}
			result.CodeOwnerReviewRequired = result.CodeOwnerReviewRequired || parameters.RequireCodeOwnerReview
			result.LastPushApprovalRequired = result.LastPushApprovalRequired || parameters.RequireLastPushApproval
			result.DismissStaleReviewsOnPush = result.DismissStaleReviewsOnPush || parameters.DismissStaleReviewsOnPush
			result.ConversationResolutionRequired = result.ConversationResolutionRequired || parameters.RequiredReviewThreadResolution
			if len(parameters.RequiredReviewers) > 0 {
				result.UnknownPolicyRules = append(result.UnknownPolicyRules, "pull_request.required_reviewers")
			}
		case "required_status_checks":
			var parameters githubRequiredStatusChecksRuleParameters
			if len(rule.Parameters) == 0 || json.Unmarshal(rule.Parameters, &parameters) != nil {
				result.UnknownPolicyRules = append(result.UnknownPolicyRules, "required_status_checks.parameters")
				continue
			}
			result.StrictStatusChecks = result.StrictStatusChecks || parameters.StrictRequiredStatusChecksPolicy
			for _, check := range parameters.RequiredStatusChecks {
				contextName := strings.TrimSpace(check.Context)
				if contextName == "" {
					result.UnknownPolicyRules = append(result.UnknownPolicyRules, "required_status_checks.empty_context")
					continue
				}
				result.RequiredStatusChecks = append(result.RequiredStatusChecks, GitHubRequiredStatusCheck{Context: contextName, IntegrationID: check.IntegrationID})
			}
		case "required_deployments":
			var parameters githubRequiredDeploymentsRuleParameters
			if len(rule.Parameters) == 0 || json.Unmarshal(rule.Parameters, &parameters) != nil {
				result.UnknownPolicyRules = append(result.UnknownPolicyRules, "required_deployments.parameters")
				continue
			}
			result.RequiredDeploymentEnvironments = append(result.RequiredDeploymentEnvironments, parameters.RequiredDeploymentEnvironments...)
		case "required_linear_history":
			result.LinearHistoryRequired = true
		case "required_signatures":
			result.RequiredSignatures = true
		case "merge_queue":
			result.MergeQueueRequired = true
		case "creation", "deletion", "non_fast_forward":
			// These rules do not add a direct PR-merge prerequisite for an
			// already-existing base ref. They remain represented by GitHub itself.
		default:
			if ruleType == "" {
				ruleType = "unknown"
			}
			result.UnknownPolicyRules = append(result.UnknownPolicyRules, ruleType)
		}
	}
}

func applyGitHubClassicProtection(result *GitHubPullRequestMergeRequirementsResult, protection githubBranchProtectionResponse) {
	if result == nil {
		return
	}
	if protection.RequiredStatusChecks != nil {
		result.StrictStatusChecks = result.StrictStatusChecks || protection.RequiredStatusChecks.Strict
		for _, contextName := range protection.RequiredStatusChecks.Contexts {
			contextName = strings.TrimSpace(contextName)
			if contextName != "" {
				result.RequiredStatusChecks = append(result.RequiredStatusChecks, GitHubRequiredStatusCheck{Context: contextName})
			}
		}
		for _, check := range protection.RequiredStatusChecks.Checks {
			contextName := strings.TrimSpace(check.Context)
			if contextName != "" {
				result.RequiredStatusChecks = append(result.RequiredStatusChecks, GitHubRequiredStatusCheck{Context: contextName, IntegrationID: check.AppID})
			}
		}
	}
	if jsonValuePresent(protection.Restrictions) {
		result.ClassicRestrictionsPresent = true
		result.UnknownPolicyRules = append(result.UnknownPolicyRules, "classic.restrictions")
	}
	if protection.RequiredPullRequestReviews != nil {
		reviews := protection.RequiredPullRequestReviews
		if reviews.RequiredApprovingReviewCount > result.RequiredApprovingReviewCount {
			result.RequiredApprovingReviewCount = reviews.RequiredApprovingReviewCount
		}
		result.CodeOwnerReviewRequired = result.CodeOwnerReviewRequired || reviews.RequireCodeOwnerReviews
		result.LastPushApprovalRequired = result.LastPushApprovalRequired || reviews.RequireLastPushApproval
		result.DismissStaleReviewsOnPush = result.DismissStaleReviewsOnPush || reviews.DismissStaleReviews
		if jsonValuePresent(reviews.BypassPullRequestAllowances) {
			result.ClassicReviewBypassAllowancesPresent = true
			result.PotentialBypass = true
			result.UnknownPolicyRules = append(result.UnknownPolicyRules, "classic.bypass_pull_request_allowances")
		}
	}
	if protection.RequiredSignatures != nil && protection.RequiredSignatures.Enabled {
		result.RequiredSignatures = true
	}
	if protection.RequiredConversationResolution != nil && protection.RequiredConversationResolution.Enabled {
		result.ConversationResolutionRequired = true
	}
	if protection.RequiredLinearHistory != nil && protection.RequiredLinearHistory.Enabled {
		result.LinearHistoryRequired = true
	}
	if protection.LockBranch != nil && protection.LockBranch.Enabled {
		result.BranchLocked = true
	}
}

func jsonValuePresent(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

func repositoryMergeMethods(settings githubRepositoryMergeSettingsResponse) []string {
	methods := make([]string, 0, 3)
	if settings.AllowMergeCommit {
		methods = append(methods, "merge")
	}
	if settings.AllowSquashMerge {
		methods = append(methods, "squash")
	}
	if settings.AllowRebaseMerge {
		methods = append(methods, "rebase")
	}
	return methods
}

func intersectMergeMethods(existing, allowed []string) []string {
	allowedSet := map[string]bool{}
	for _, method := range allowed {
		method = strings.ToLower(strings.TrimSpace(method))
		switch method {
		case "merge", "squash", "rebase":
			allowedSet[method] = true
		}
	}
	out := make([]string, 0, len(existing))
	for _, method := range existing {
		if allowedSet[method] {
			out = append(out, method)
		}
	}
	return out
}

func removeString(values []string, remove string) []string {
	out := values[:0]
	for _, value := range values {
		if value != remove {
			out = append(out, value)
		}
	}
	return out
}

func sortedUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeGitHubRequiredStatusChecks(values []GitHubRequiredStatusCheck) []GitHubRequiredStatusCheck {
	seen := map[string]bool{}
	out := make([]GitHubRequiredStatusCheck, 0, len(values))
	for _, value := range values {
		contextName := strings.TrimSpace(value.Context)
		if contextName == "" {
			continue
		}
		integration := int64(0)
		if value.IntegrationID != nil {
			integration = *value.IntegrationID
		}
		key := fmt.Sprintf("%s\x00%d", contextName, integration)
		if seen[key] {
			continue
		}
		seen[key] = true
		value.Context = contextName
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Context != out[j].Context {
			return out[i].Context < out[j].Context
		}
		left, right := int64(0), int64(0)
		if out[i].IntegrationID != nil {
			left = *out[i].IntegrationID
		}
		if out[j].IntegrationID != nil {
			right = *out[j].IntegrationID
		}
		return left < right
	})
	return out
}

func (s *RemoteService) doGitHubReadJSONStatus(ctx context.Context, token, endpoint string, out interface{}) (int, error) {
	if s == nil || s.githubClient == nil || !strings.HasPrefix(endpoint, "/repos/") {
		return 0, fmt.Errorf("GitHub API request is unavailable")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIBaseURL+endpoint, nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("User-Agent", "OmniLLM-Studio")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := s.githubClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || out == nil {
		return response.StatusCode, nil
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return response.StatusCode, err
	}
	return response.StatusCode, nil
}
