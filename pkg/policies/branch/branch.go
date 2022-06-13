// Copyright 2021 Allstar Authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package branch implements the Branch Protection security policy.
package branch

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v43/github"
	"github.com/rs/zerolog/log"
)

const configFile = "branch_protection.yaml"
const polName = "Branch Protection"

// OrgConfig is the org-level config definition for Branch Protection.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride
	// applies to all BP config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`

	// EnforceDefault : set to true to enforce policy on default branch, default
	// true.
	EnforceDefault bool `json:"enforceDefault"`

	// EnforceBranches is a map of repos and branches. These are other
	// non-default branches to enforce policy on, such as branches which releases
	// are made from.
	EnforceBranches map[string][]string `json:"enforceBranches"`

	// RequireApproval : set to true to enforce approval on PRs, default true.
	RequireApproval bool `json:"requireApproval"`

	// ApprovalCount is the number of required PR approvals, default 1.
	ApprovalCount int `json:"approvalCount"`

	// DismissStale : set to true to require PR approvals be dismissed when a PR
	// is updated, default true.
	DismissStale bool `json:"dismissStale"`

	// BlockForce : set to true to block force pushes, default true.
	BlockForce bool `json:"blockForce"`

	// RequireUpToDateBranch : set to true to require that branches must be up
	// to date before merging. Only used if RequireStatusChecks is set. Default
	// true.
	RequireUpToDateBranch bool `json:"requireUpToDateBranch"`

	// RequireStatusChecks is a list of status checks that are required in
	// order to merge into the protected branch. Each entry must specify
	// the context, and optionally an appID.
	RequireStatusChecks []StatusCheck `json:"requireStatusChecks"`

	// EnforceOnAdmins : set to true to apply the branch protection rules on
	// administrators as well.
	EnforceOnAdmins bool `json:"enforceOnAdmins"`

	// RequireSignedCommits : set to true to require signed commits on protected branches, default false
	RequireSignedCommits bool `json:"requireSignedCommits"`
}

// RepoConfig is the repo-level config for Branch Protection
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `json:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `json:"action"`

	// EnforceDefault overrides the same setting in org-level, only if present.
	EnforceDefault *bool `json:"enforceDefault"`

	// EnforceBranches adds more branches to the org-level list. Does not
	// override. Always allowed irrespective of DisableRepoOverride setting.
	EnforceBranches []string `json:"enforceBranches"`

	// RequireApproval overrides the same setting in org-level, only if present.
	RequireApproval *bool `json:"requireApproval"`

	// ApprovalCount overrides the same setting in org-level, only if present.
	ApprovalCount *int `json:"approvalCount"`

	// DismissStale overrides the same setting in org-level, only if present.
	DismissStale *bool `json:"dismissStale"`

	// BlockForce overrides the same setting in org-level, only if present.
	BlockForce *bool `json:"blockForce"`

	// EnforceOnAdmins overrides the same setting in org-level, only if present.
	EnforceOnAdmins *bool `json:"enforceOnAdmins"`

	// RequireUpToDateBranch overrides the same setting in org-level, only if
	// present.
	RequireUpToDateBranch *bool `json:"requireUpToDateBranch"`

	// RequireStatusChecks overrides the same setting in org-level, only if
	// present. Omitting will lead to taking the org-level config as is, but
	// specifying an empty list (`requireStatusChecks: []`) will override the
	// setting to be empty.
	RequireStatusChecks []StatusCheck `json:"requireStatusChecks"`

	// RequireSignedCommits overrides the same setting in org-level, only if
	// present.
	RequireSignedCommits *bool `json:"requireSignedCommits"`
}

// StatusCheck is the config description for specifying a single required
// status check in the RequireStatusChecks list.
type StatusCheck struct {
	// Context is the status check name that should be required.
	Context string `json:"context"`

	// AppID, when provided, will require that the status check be set by
	// the GitHub App with the given AppID. When omitted, any app can
	// provide the required status check.
	AppID *int64 `json:"appID"`
}

type statusCheckHash struct {
	context string
	appID   int64
}

type mergedConfig struct {
	Action                string
	EnforceDefault        bool
	EnforceBranches       []string
	RequireApproval       bool
	ApprovalCount         int
	DismissStale          bool
	BlockForce            bool
	EnforceOnAdmins       bool
	RequireUpToDateBranch bool
	RequireStatusChecks   []StatusCheck
	RequireSignedCommits  bool
}

type details struct {
	PRReviews             bool
	NumReviews            int
	DismissStale          bool
	BlockForce            bool
	EnforceOnAdmins       bool
	RequireUpToDateBranch bool
	RequireStatusChecks   []StatusCheck
	RequireSignedCommits  bool
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error
var configIsEnabled func(ctx context.Context, o config.OrgOptConfig,
	orc, r config.RepoOptConfig, c *github.Client, owner, repo string) (bool, error)
var doNothingOnOptOut = operator.DoNothingOnOptOut

func init() {
	configFetchConfig = config.FetchConfig
	configIsEnabled = config.IsEnabled
}

// Branch is the Branch Protection policy object, implements policydef.Policy.
type Branch bool

// NewBranch returns a new BranchProtection polcy.
func NewBranch() policydef.Policy {
	var b Branch
	return b
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (b Branch) Name() string {
	return polName
}

type repositories interface {
	Get(context.Context, string, string) (*github.Repository,
		*github.Response, error)
	ListBranches(context.Context, string, string, *github.BranchListOptions) (
		[]*github.Branch, *github.Response, error)
	GetBranchProtection(context.Context, string, string, string) (
		*github.Protection, *github.Response, error)
	UpdateBranchProtection(context.Context, string, string, string,
		*github.ProtectionRequest) (*github.Protection, *github.Response, error)
	GetSignaturesProtectedBranch(ctx context.Context, owner, repo, branch string) (
		*github.SignaturesProtectedBranch, *github.Response, error)
	RequireSignaturesOnProtectedBranch(ctx context.Context, owner, repo, branch string) (
		*github.SignaturesProtectedBranch, *github.Response, error)
}

// Check performs the polcy check for Branch Protection based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (b Branch) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	return check(ctx, c.Repositories, c, owner, repo)
}

func check(ctx context.Context, rep repositories, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	enabled, err := configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
	if err != nil {
		return nil, err
	}
	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Bool("enabled", enabled).
		Msg("Check repo enabled")
	if !enabled && doNothingOnOptOut {
		// Don't run this policy if disabled and requested by operator. This is
		// only checking enablement of policy, but not Allstar overall, this is
		// ok for now.
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       true,
			NotifyText: "Disabled",
			Details:    map[string]details{},
		}, nil
	}

	mc := mergeConfig(oc, orc, rc, repo)

	r, _, err := rep.Get(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	opt := &github.BranchListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	var branches []*github.Branch
	for {
		bs, resp, err := rep.ListBranches(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		branches = append(branches, bs...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	// Don't really need pagination here, only checking if no branches exist.
	if len(branches) == 0 {
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       true,
			NotifyText: "No branches to protect",
			Details:    nil,
		}, nil
	}

	allBranches := mc.EnforceBranches
	if mc.EnforceDefault {
		allBranches = append(mc.EnforceBranches, r.GetDefaultBranch())
	}
	if len(allBranches) == 0 {
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       true,
			NotifyText: "No branches configured for enforcement in policy",
			Details:    nil,
		}, nil
	}
	pass := true
	text := ""
	ds := make(map[string]details)
	for _, b := range allBranches {
		p, rsp, err := rep.GetBranchProtection(ctx, owner, repo, b)
		if err != nil {
			if rsp != nil && rsp.StatusCode == http.StatusNotFound {
				// Branch not protected
				pass = false
				text = text + fmt.Sprintf("No protection found for branch %v\n", b)
				ds[b] = details{}
				continue
			}
			if rsp != nil && rsp.StatusCode == http.StatusForbidden {
				// Protection not available
				pass = false
				text = text + "Branch Protection enforcement is configured in Allstar, however Branch Protection is not available on this repository. Upgrade to GitHub Pro or make this repository public to enable this feature.\n" +
					"See: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/defining-the-mergeability-of-pull-requests/about-protected-branches for more information.\n" +
					"If this is not feasible, then disable Branch Protection policy enforcement for this repository in Allstar configuration."
				ds[b] = details{}
				break
			}
			return nil, err
		}

		var d details
		rev := p.GetRequiredPullRequestReviews()
		if rev != nil {
			d.PRReviews = true
			d.DismissStale = rev.DismissStaleReviews
			if mc.DismissStale && !rev.DismissStaleReviews {
				text = text +
					fmt.Sprintf("Dismiss stale reviews not configured for branch %v\n", b)
				pass = false
			}
			d.NumReviews = rev.RequiredApprovingReviewCount
			if rev.RequiredApprovingReviewCount < mc.ApprovalCount {
				pass = false
				text = text +
					fmt.Sprintf("PR Approvals below threshold %v : %v for branch %v\n",
						rev.RequiredApprovingReviewCount, mc.ApprovalCount, b)
			}
		} else {
			if mc.RequireApproval {
				pass = false
				text = text +
					fmt.Sprintf("PR Approvals not configured for branch %v\n", b)
			}
		}
		afp := p.GetAllowForcePushes()
		d.BlockForce = true
		if afp != nil {
			if mc.BlockForce && afp.Enabled {
				text = text +
					fmt.Sprintf("Block force push not configured for branch %v\n", b)
				pass = false
				d.BlockForce = false
			}
		}
		ea := p.GetEnforceAdmins()
		d.EnforceOnAdmins = (ea != nil && ea.Enabled)
		if mc.EnforceOnAdmins && (ea == nil || !ea.Enabled) {
			text = text +
				fmt.Sprintf("Enforce status checks on admins not configured for branch %v\n",
					b)
			pass = false
		}
		if len(mc.RequireStatusChecks) > 0 {
			rsc := p.GetRequiredStatusChecks()
			if rsc != nil {
				d.RequireUpToDateBranch = rsc.Strict
				if mc.RequireUpToDateBranch && !rsc.Strict {
					text = text +
						fmt.Sprintf("Require up to date branch not configured for branch %v\n",
							b)
					pass = false
				}
				for _, c := range rsc.Checks {
					sc := StatusCheck{Context: c.Context, AppID: c.AppID}
					d.RequireStatusChecks = append(d.RequireStatusChecks, sc)
				}
				lt := makeSCLookupTable(rsc.Checks)
				for _, c := range mc.RequireStatusChecks {
					appIDTxt := "(any app)"
					sch := statusCheckHash{context: c.Context}
					if c.AppID != nil {
						sch.appID = *c.AppID
						appIDTxt = fmt.Sprintf("(AppID: %v)", *c.AppID)
					}
					if _, ok := lt[sch]; !ok {
						text = text +
							fmt.Sprintf("Status check %s %s not found for branch %v\n",
								c.Context, appIDTxt, b)
						pass = false
					}
				}
			} else {
				text = text +
					fmt.Sprintf("Status checks required by policy, but none found for branch %v\n", b)
				pass = false
			}
		}

		signatureProtectionEnabled, err := getSignatureProtectionEnabled(ctx, rep, owner, repo, b)
		if err != nil {
			return nil, err
		}
		d.RequireSignedCommits = signatureProtectionEnabled
		if mc.RequireSignedCommits && !d.RequireSignedCommits {
			pass = false
			text = text + fmt.Sprintf("Signed commits required, but not enabled for branch: %v\n", b)
		}

		ds[b] = d
	}

	return &policydef.Result{
		Enabled:    enabled,
		Pass:       pass,
		NotifyText: text,
		Details:    ds,
	}, nil
}

// Fix implementing policydef.Policy.Fix().
func (b Branch) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	return fix(ctx, c.Repositories, c, owner, repo)
}

func fix(ctx context.Context, rep repositories, c *github.Client,
	owner, repo string) error {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	enabled, err := configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	mc := mergeConfig(oc, orc, rc, repo)

	r, _, err := rep.Get(ctx, owner, repo)
	if err != nil {
		return err
	}
	allBranches := mc.EnforceBranches
	if mc.EnforceDefault {
		allBranches = append(mc.EnforceBranches, r.GetDefaultBranch())
	}
	for _, b := range allBranches {
		p, rsp, err := rep.GetBranchProtection(ctx, owner, repo, b)
		if err != nil {
			if rsp != nil && rsp.StatusCode == http.StatusNotFound {
				// No existing protection, create from config.
				afp := !mc.BlockForce
				pr := &github.ProtectionRequest{
					AllowForcePushes: &afp,
				}
				if mc.EnforceOnAdmins {
					pr.EnforceAdmins = true
				}
				if mc.RequireApproval {
					rq := &github.PullRequestReviewsEnforcementRequest{
						DismissStaleReviews:          mc.DismissStale,
						RequiredApprovingReviewCount: mc.ApprovalCount,
					}
					pr.RequiredPullRequestReviews = rq
				}
				if len(mc.RequireStatusChecks) > 0 {
					checks := make([]*github.RequiredStatusCheck, len(mc.RequireStatusChecks))
					for i, check := range mc.RequireStatusChecks {
						checks[i] = &github.RequiredStatusCheck{
							Context: check.Context,
							AppID:   check.AppID,
						}
					}
					rsc := &github.RequiredStatusChecks{
						Strict: mc.RequireUpToDateBranch,
						Checks: checks,
					}
					pr.RequiredStatusChecks = rsc
				}
				_, rsp, err := rep.UpdateBranchProtection(ctx, owner, repo, b, pr)
				if err != nil {
					if rsp != nil && rsp.StatusCode == http.StatusForbidden {
						log.Warn().
							Str("org", owner).
							Str("repo", repo).
							Str("area", polName).
							Msg("Action set to fix, but did not accept admin:write permissions update.")
						// no sense to continue, just return
						return nil
					}
					return err
				}
				continue
			}
			if rsp != nil && rsp.StatusCode == http.StatusForbidden {
				log.Warn().
					Str("org", owner).
					Str("repo", repo).
					Str("area", polName).
					Msg("Fix action selected, but repo does not support Branch Protection.")
				// no sense to continue, just return
				return nil
			}
			return err
		}
		// Got existing protection, modify from existing
		update := false
		pr := &github.ProtectionRequest{
			RequiredStatusChecks: p.RequiredStatusChecks,
			EnforceAdmins:        p.EnforceAdmins.Enabled,
			AllowForcePushes:     &p.AllowForcePushes.Enabled,
		}
		if mc.EnforceOnAdmins && !pr.EnforceAdmins {
			pr.EnforceAdmins = true
			update = true
		}
		if pr.RequiredStatusChecks != nil {
			// Clear out Contexts, since API populates both, but updates require only one.
			pr.RequiredStatusChecks.Contexts = nil
			// If there are no actual checks or contexts, then unset RequiredStatusChecks entirely,
			// otherwise update fails
			if len(pr.RequiredStatusChecks.Checks) == 0 && len(pr.RequiredStatusChecks.Contexts) == 0 {
				update = true
				pr.RequiredStatusChecks = nil
			}
		}
		if p.RequiredPullRequestReviews != nil {
			prr := &github.PullRequestReviewsEnforcementRequest{
				DismissStaleReviews:          p.RequiredPullRequestReviews.DismissStaleReviews,
				RequireCodeOwnerReviews:      p.RequiredPullRequestReviews.RequireCodeOwnerReviews,
				RequiredApprovingReviewCount: p.RequiredPullRequestReviews.RequiredApprovingReviewCount,
			}
			pr.RequiredPullRequestReviews = prr
		}
		if p.Restrictions != nil {
			rr := &github.BranchRestrictionsRequest{
				Users: make([]string, 0),
				Teams: make([]string, 0),
			}
			if p.Restrictions.Users != nil {
				for _, u := range p.Restrictions.Users {
					rr.Users = append(rr.Users, *u.Login)
				}
			}
			if p.Restrictions.Teams != nil {
				for _, t := range p.Restrictions.Teams {
					rr.Teams = append(rr.Teams, *t.Slug)
				}
			}
			if p.Restrictions.Apps != nil {
				rr.Apps = make([]string, 0)
				for _, a := range p.Restrictions.Apps {
					rr.Apps = append(rr.Apps, *a.Slug)
				}
			}
			pr.Restrictions = rr
		}
		if *pr.AllowForcePushes && mc.BlockForce {
			f := false
			pr.AllowForcePushes = &f
			update = true
		}
		if pr.RequiredPullRequestReviews == nil && mc.RequireApproval {
			rq := &github.PullRequestReviewsEnforcementRequest{
				DismissStaleReviews:          mc.DismissStale,
				RequiredApprovingReviewCount: mc.ApprovalCount,
			}
			pr.RequiredPullRequestReviews = rq
			update = true
		}
		if mc.RequireApproval {
			if mc.DismissStale && !pr.RequiredPullRequestReviews.DismissStaleReviews {
				pr.RequiredPullRequestReviews.DismissStaleReviews = true
				update = true
			}
			if mc.ApprovalCount > pr.RequiredPullRequestReviews.RequiredApprovingReviewCount {
				pr.RequiredPullRequestReviews.RequiredApprovingReviewCount = mc.ApprovalCount
				update = true
			}
		}
		if len(mc.RequireStatusChecks) > 0 {
			if pr.RequiredStatusChecks == nil {
				checks := make([]*github.RequiredStatusCheck, len(mc.RequireStatusChecks))
				for i, check := range mc.RequireStatusChecks {
					checks[i] = &github.RequiredStatusCheck{
						Context: check.Context,
						AppID:   check.AppID,
					}
				}
				rsc := &github.RequiredStatusChecks{
					Strict: mc.RequireUpToDateBranch,
					Checks: checks,
				}
				pr.RequiredStatusChecks = rsc
				update = true
			} else {
				if mc.RequireUpToDateBranch && !pr.RequiredStatusChecks.Strict {
					pr.RequiredStatusChecks.Strict = true
					update = true
				}
				ac := pr.RequiredStatusChecks.Checks
				lt := makeSCLookupTable(pr.RequiredStatusChecks.Checks)
				for _, c := range mc.RequireStatusChecks {
					// Only mark for update if there are status checks required, but not already set.
					sch := statusCheckHash{context: c.Context}
					if c.AppID != nil {
						sch.appID = *c.AppID
					}
					if _, ok := lt[sch]; !ok {
						ac = append(ac, &github.RequiredStatusCheck{Context: c.Context, AppID: c.AppID})
						update = true
					}
				}
				pr.RequiredStatusChecks.Checks = ac
			}
		}
		if update {
			_, rsp, err := rep.UpdateBranchProtection(ctx, owner, repo, b, pr)
			if err != nil {
				if rsp != nil && rsp.StatusCode == http.StatusForbidden {
					log.Warn().
						Str("org", owner).
						Str("repo", repo).
						Str("area", polName).
						Msg("Action set to fix, but did not accept admin:write permissions update.")
					return nil
				}
				return err
			}
			log.Info().
				Str("org", owner).
				Str("repo", repo).
				Str("area", polName).
				Msg("Updated with Fix action.")
		}

		signatureProtectionEnabled, err := getSignatureProtectionEnabled(ctx, rep, owner, repo, b)
		if err != nil {
			return err
		}
		if mc.RequireSignedCommits && !signatureProtectionEnabled {
			_, rsp, err = rep.RequireSignaturesOnProtectedBranch(ctx, owner, repo, b)

			if err != nil {
				if rsp != nil && rsp.StatusCode == http.StatusForbidden {
					log.Warn().
						Str("org", owner).
						Str("repo", repo).
						Str("area", polName).
						Str("branch", b).
						Msg("Action set to fix, but did not accept admin:write update to make signed commits required.")
					return nil
				}
				return err
			}
			log.Info().
				Str("org", owner).
				Str("repo", repo).
				Str("area", polName).
				Str("branch", b).
				Msg("Updated to make signed commits required with Fix action.")
		}
	}
	return nil
}

func getSignatureProtectionEnabled(ctx context.Context, rep repositories, owner string, repo string, branch string) (
	bool, error) {
	sp, rsp, err := rep.GetSignaturesProtectedBranch(ctx, owner, repo, branch)
	if err != nil {
		if rsp != nil && rsp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		if rsp != nil && rsp.StatusCode == http.StatusForbidden {
			err = fmt.Errorf("access denied to commit signing settings for %v: %w", repo, err)
		}
		return false, err
	}
	return sp.GetEnabled(), nil
}

// GetAction returns the configured action from Branch Protection's
// configuration stored in the org-level repo, default log. Implementing
// policydef.Policy.GetAction()
func (b Branch) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, orc, rc, repo)
	return mc.Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig, *RepoConfig) {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action:                "log",
		EnforceDefault:        true,
		RequireApproval:       true,
		ApprovalCount:         1,
		DismissStale:          true,
		BlockForce:            true,
		RequireUpToDateBranch: true,
	}
	if err := configFetchConfig(ctx, c, owner, "", configFile, config.OrgLevel, oc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Str("configLevel", "orgLevel").
			Str("area", polName).
			Str("file", configFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	orc := &RepoConfig{}
	if err := configFetchConfig(ctx, c, owner, repo, configFile, config.OrgRepoLevel, orc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Str("configLevel", "orgRepoLevel").
			Str("area", polName).
			Str("file", configFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	rc := &RepoConfig{}
	if err := configFetchConfig(ctx, c, owner, repo, configFile, config.RepoLevel, rc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Str("configLevel", "repoLevel").
			Str("area", polName).
			Str("file", configFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	return oc, orc, rc
}

func mergeConfig(oc *OrgConfig, orc, rc *RepoConfig, repo string) *mergedConfig {
	mc := &mergedConfig{
		Action:                oc.Action,
		EnforceDefault:        oc.EnforceDefault,
		EnforceBranches:       oc.EnforceBranches[repo],
		RequireApproval:       oc.RequireApproval,
		ApprovalCount:         oc.ApprovalCount,
		DismissStale:          oc.DismissStale,
		BlockForce:            oc.BlockForce,
		EnforceOnAdmins:       oc.EnforceOnAdmins,
		RequireUpToDateBranch: oc.RequireUpToDateBranch,
		RequireStatusChecks:   oc.RequireStatusChecks,
		RequireSignedCommits:  oc.RequireSignedCommits,
	}
	mc.EnforceBranches = append(mc.EnforceBranches, orc.EnforceBranches...)
	mc = mergeInRepoConfig(mc, orc, repo)

	mc.EnforceBranches = append(mc.EnforceBranches, rc.EnforceBranches...)
	if !oc.OptConfig.DisableRepoOverride {
		mc = mergeInRepoConfig(mc, rc, repo)
	}
	return mc
}

func mergeInRepoConfig(mc *mergedConfig, rc *RepoConfig, repo string) *mergedConfig {
	if rc.Action != nil {
		mc.Action = *rc.Action
	}
	if rc.EnforceDefault != nil {
		mc.EnforceDefault = *rc.EnforceDefault
	}
	if rc.RequireApproval != nil {
		mc.RequireApproval = *rc.RequireApproval
	}
	if rc.ApprovalCount != nil {
		mc.ApprovalCount = *rc.ApprovalCount
	}
	if rc.DismissStale != nil {
		mc.DismissStale = *rc.DismissStale
	}
	if rc.BlockForce != nil {
		mc.BlockForce = *rc.BlockForce
	}
	if rc.EnforceOnAdmins != nil {
		mc.EnforceOnAdmins = *rc.EnforceOnAdmins
	}
	if rc.RequireUpToDateBranch != nil {
		mc.RequireUpToDateBranch = *rc.RequireUpToDateBranch
	}
	if rc.RequireStatusChecks != nil {
		mc.RequireStatusChecks = rc.RequireStatusChecks
	}
	if rc.RequireSignedCommits != nil {
		mc.RequireSignedCommits = *rc.RequireSignedCommits
	}
	return mc
}

func makeSCLookupTable(prrsc []*github.RequiredStatusCheck) map[statusCheckHash]struct{} {
	lt := make(map[statusCheckHash]struct{}, len(prrsc))
	for _, c := range prrsc {
		// each check can match the context with and without appID
		sc := statusCheckHash{context: c.Context}
		lt[sc] = struct{}{}
		if c.AppID != nil {
			lt[statusCheckHash{context: c.Context, appID: *c.AppID}] = struct{}{}
		}
	}
	return lt
}
