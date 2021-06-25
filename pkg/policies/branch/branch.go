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
	"path"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v35/github"
	"github.com/rs/zerolog/log"
)

const configFile = "branch_protection.yaml"
const polName = "Branch Protection"

// OrgConfig is the org-level config definition for Branch Protection.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride applies to all
	// BP config.
	OptConfig config.OrgOptConfig `yaml:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `yaml:"action"`

	// EnforceDefault : set to true to enforce policy on default branch, default true.
	EnforceDefault bool `yaml:"enforceDefault"`

	// EnforceBranches is a map of repos and branches. These are other
	// non-default branches to enforce policy on, such as branches which releases
	// are made from.
	EnforceBranches map[string][]string `yaml:"enforceBranches"`

	// RequireApproval : set to true to enforce approval on PRs, default true.
	RequireApproval bool `yaml:"requireApproval"`

	// ApprovalCount is the number of required PR approvals, default 1.
	ApprovalCount int `yaml:"approvalCount"`

	// DismissStale : set to true to require PR approvalse be dismissed when a PR is updated, default true.
	DismissStale bool `yaml:"dismissStale"`

	// BlockForce : set to true to block force pushes, default true.
	BlockForce bool `yaml:"blockForce"`
}

// RepoConfig is the repo-level config for Branch Protection
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `yaml:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `yaml:"action"`

	// EnforceDefault overrides the same setting in org-level, only if present.
	EnforceDefault *bool `yaml:"enforceDefault"`

	// EnforceBranches adds more branches to the org-level list. Does not
	// override. Always allowed irrespective of DisableRepoOverride setting.
	EnforceBranches []string `yaml:"enforceBranches"`

	// RequireApproval overrides the same setting in org-level, only if present.
	RequireApproval *bool `yaml:"requireAppproval"`

	// ApprovalCount overrides the same setting in org-level, only if present.
	ApprovalCount *int `yaml:"approvalCount"`

	// DismissStale overrides the same setting in org-level, only if present.
	DismissStale *bool `yaml:"dismissStale"`

	// BlockForce overrides the same setting in org-level, only if present.
	BlockForce *bool `yaml:"blockForce"`
}

type mergedConfig struct {
	Action          string
	EnforceDefault  bool
	EnforceBranches []string
	RequireApproval bool
	ApprovalCount   int
	DismissStale    bool
	BlockForce      bool
}

type details struct {
	PRReviews    bool
	NumReviews   int
	DismissStale bool
	BlockForce   bool
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, interface{}) error

func init() {
	configFetchConfig = config.FetchConfig
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
}

// Check performs the polcy check for Branch Protection based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (b Branch) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	return check(ctx, c.Repositories, c, owner, repo)
}

func check(ctx context.Context, rep repositories, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	oc, rc := getConfig(ctx, c, owner, repo)
	enabled := config.IsEnabled(oc.OptConfig, rc.OptConfig, repo)
	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Bool("enabled", enabled).
		Msg("Check repo enabled")
	mc := mergeConfig(oc, rc, repo)

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
	ds := make(map[string]details, 0)
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
		ds[b] = d
	}

	return &policydef.Result{
		Enabled:    enabled,
		Pass:       pass,
		NotifyText: text,
		Details:    ds,
	}, nil
}

// Fix implementing policydef.Policy.Fix(). Currently not supported. BP plans
// to support this TODO.
func (b Branch) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	log.Warn().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Msg("Action fix is configured, but not implemented.")
	return nil
}

// GetAction returns the configured action from Branch Protection's
// configuration stored in the org-level repo, default log. Implementing
// policydef.Policy.GetAction()
func (b Branch) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	// drop errors, if cfg file is not there, go with defaults
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action: "log",
	}
	configFetchConfig(ctx, c, owner, operator.OrgConfigRepo, configFile, oc)
	return oc.Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig) {
	// drop errors, if cfg file is not there, go with defaults
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action:          "log",
		EnforceDefault:  true,
		RequireApproval: true,
		ApprovalCount:   1,
		DismissStale:    true,
		BlockForce:      true,
	}
	configFetchConfig(ctx, c, owner, operator.OrgConfigRepo, configFile, oc)
	rc := &RepoConfig{}
	configFetchConfig(ctx, c, owner, repo, path.Join(operator.RepoConfigDir, configFile), rc)
	return oc, rc
}

func mergeConfig(oc *OrgConfig, rc *RepoConfig, repo string) *mergedConfig {
	mc := &mergedConfig{
		Action:          oc.Action,
		EnforceDefault:  oc.EnforceDefault,
		EnforceBranches: oc.EnforceBranches[repo],
		RequireApproval: oc.RequireApproval,
		ApprovalCount:   oc.ApprovalCount,
		DismissStale:    oc.DismissStale,
		BlockForce:      oc.BlockForce,
	}
	mc.EnforceBranches = append(mc.EnforceBranches, rc.EnforceBranches...)

	if !oc.OptConfig.DisableRepoOverride {
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
	}
	return mc
}
