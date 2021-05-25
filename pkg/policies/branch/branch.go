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

// Package branch implement branch protection security policies
package branch

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v35/github"
)

const config_ConfigFile = "branch_protection.yaml"

type OrgConfig struct {
	OptConfig       config.OrgOptConfig `yaml:"optConfig"`       // Standard opt in/out config, RepoOverride applies to all config
	Action          string              `yaml:"action"`          // Which action to take, default log, other: issue, block
	EnforceDefault  bool                `yaml:"enforceDefault"`  // Enforce policy on default branch, default true
	EnforceBranches map[string][]string `yaml:"enforceBranches"` // Other branches to enforce policy on, key is repo name
	RequireApproval bool                `yaml:"requireApproval"` // Enforce approval required
	ApprovalCount   int                 `yaml:"approvalCount"`   // Number of approvals expected
	DismissStale    bool                `yaml:"dismissStale"`    // Enforce dismiss stale
	BlockForce      bool                `yaml:"blockForce"`      // Enforce blocking force push
}

type RepoConfig struct {
	OptConfig       config.RepoOptConfig `yaml:"optConfig"`        // Standard opt in/out config
	EnforceDefault  *bool                `yaml:"enforceDefault"`   // If present, override same org config
	EnforceBranches []string             `yaml:"enforceBranches"`  // Additive to any branches in org config, always allowed
	RequireApproval *bool                `yaml:"requireAppproval"` // If present, override same org config
	ApprovalCount   *int                 `yaml:"approvalCount"`    // If present, override same org config
	DismissStale    *bool                `yaml:"dismissStale"`     // If present, override same org config
	BlockForce      *bool                `yaml:"blockForce"`       // If present, override same org config
}

type MergedConfig struct {
	Action          string
	EnforceDefault  bool
	EnforceBranches []string
	RequireApproval bool
	ApprovalCount   int
	DismissStale    bool
	BlockForce      bool
}

type Details struct {
	PRReviews    bool
	NumReviews   int
	DismissStale bool
	BlockForce   bool
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, interface{}) error

func init() {
	configFetchConfig = config.FetchConfig
}

type Branch bool

func NewBranch() policydef.Policy {
	var b Branch
	return b
}

func (b Branch) Name() string {
	return "Branch protection"
}

type repositories interface {
	Get(context.Context, string, string) (*github.Repository,
		*github.Response, error)
	ListBranches(context.Context, string, string, *github.BranchListOptions) (
		[]*github.Branch, *github.Response, error)
	GetBranchProtection(context.Context, string, string, string) (
		*github.Protection, *github.Response, error)
}

func (b Branch) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	return check(ctx, c.Repositories, c, owner, repo)
}

func check(ctx context.Context, rep repositories, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	oc, rc := getConfig(ctx, c, owner, repo)
	enabled := config.IsEnabled(oc.OptConfig, rc.OptConfig, repo)
	log.Printf("Repo branch protection enabled? %v / %v : %v", owner, repo, enabled)
	if !enabled {
		return &policydef.Result{
			Pass:       true,
			NotifyText: "Disabled",
			Details:    nil,
		}, nil
	}
	mc := mergeConfig(oc, rc, repo)

	r, _, err := rep.Get(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	bs, _, err := rep.ListBranches(ctx, owner, repo, nil)
	if err != nil {
		return nil, err
	}
	if len(bs) == 0 {
		return &policydef.Result{
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
			Pass:       true,
			NotifyText: "No branches configured for enforcement in policy",
			Details:    nil,
		}, nil
	}
	pass := true
	text := ""
	details := make(map[string]Details, 0)
	for _, b := range allBranches {
		p, rsp, err := rep.GetBranchProtection(ctx, owner, repo, b)
		if err != nil {
			if rsp != nil && rsp.StatusCode == http.StatusNotFound {
				// Branch not protected
				pass = false
				text = text + fmt.Sprintf("No protection found for branch %v\n", b)
				details[b] = Details{}
				continue
			}
			return nil, err
		}

		var d Details
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
		details[b] = d
	}

	return &policydef.Result{
		Pass:       pass,
		NotifyText: text,
		Details:    details,
	}, nil
}

func (b Branch) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	log.Printf("Action fix is not implemented for policy %v", b.Name())
	return nil
}

func (b Branch) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	// drop errors, if cfg file is not there, go with defaults
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action: "log",
	}
	configFetchConfig(ctx, c, owner, config.GetOrgRepo(), config_ConfigFile, oc)
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
	configFetchConfig(ctx, c, owner, config.GetOrgRepo(), config_ConfigFile, oc)
	rc := &RepoConfig{}
	configFetchConfig(ctx, c, owner, repo, path.Join(config.GetRepoDir(), config_ConfigFile), rc)
	return oc, rc
}

func mergeConfig(oc *OrgConfig, rc *RepoConfig, repo string) *MergedConfig {
	mc := &MergedConfig{
		Action:          oc.Action,
		EnforceDefault:  oc.EnforceDefault,
		EnforceBranches: oc.EnforceBranches[repo],
		RequireApproval: oc.RequireApproval,
		ApprovalCount:   oc.ApprovalCount,
		DismissStale:    oc.DismissStale,
		BlockForce:      oc.BlockForce,
	}
	mc.EnforceBranches = append(mc.EnforceBranches, rc.EnforceBranches...)
	// FIXME if repo override
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
	return mc
}
