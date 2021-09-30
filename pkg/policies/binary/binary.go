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

// Package binary implements the Binary Artifacts security policy check from
// scorecard.
package binary

import (
	"context"
	"fmt"
	"path"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/configdef"	
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/policydef"

	gh32 "github.com/google/go-github/v32/github"
	"github.com/google/go-github/v39/github"
	"github.com/ossf/scorecard/checker"
	"github.com/ossf/scorecard/checks"
	"github.com/ossf/scorecard/clients/githubrepo"
	"github.com/rs/zerolog/log"
)

const configFile = "binary_artifacts.yaml"
const polName = "Binary Artifacts"

// OrgConfig is the org-level config definition for this policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride applies to all
	// config.
	OptConfig config.OrgOptConfig `yaml:"optConfig"`

	ActionConfig config.OrgActionConfig `yaml:"actionConfig"`
	
	// Action defines which action to take, default log, other: issue...
	Action string `yaml:"action"`
}

// RepoConfig is the repo-level config for this policy.
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `yaml:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `yaml:"action"`

	ActionConfig config.OrgActionConfig `yaml:"actionConfig"`
}

type mergedConfig struct {
	Action string
	ActionConfig configdef.OrgActionConfig
}

type details struct {
	Messages []checker.CheckDetail
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, interface{}) error

func init() {
	configFetchConfig = config.FetchConfig
}

// Binary is the Binary Artifacts policy object, implements policydef.Policy.
type Binary bool

// NewBinary returns a new Binary Artifacts policy.
func NewBinary() policydef.Policy {
	var b Binary
	return b
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (b Binary) Name() string {
	return polName
}

type logger struct {
	Messages2 []checker.CheckDetail
}

func (l *logger) Info(desc string, args ...interface{}) {
	cd := checker.CheckDetail{Type: checker.DetailInfo, Msg: fmt.Sprintf(desc, args...)}
	l.Messages2 = append(l.Messages2, cd)
}

func (l *logger) Warn(desc string, args ...interface{}) {
	cd := checker.CheckDetail{Type: checker.DetailWarn, Msg: fmt.Sprintf(desc, args...)}
	l.Messages2 = append(l.Messages2, cd)
}

func (l *logger) Debug(desc string, args ...interface{}) {
	cd := checker.CheckDetail{Type: checker.DetailDebug, Msg: fmt.Sprintf(desc, args...)}
	l.Messages2 = append(l.Messages2, cd)
}

// Check performs the polcy check for this policy based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (b Binary) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	oc, rc := getConfig(ctx, c, owner, repo)
	enabled := config.IsEnabled(oc.OptConfig, rc.OptConfig, repo)
	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Bool("enabled", enabled).
		Msg("Check repo enabled")

	oldClient := gh32.NewClient(c.Client())
	repoClient := githubrepo.CreateGithubRepoClient(ctx, oldClient)
	if err := repoClient.InitRepo(owner, repo); err != nil {
		return nil, err
	}
	defer repoClient.Close()
	l := logger{}
	cr := &checker.CheckRequest{
		Ctx:         ctx,
		Client:      oldClient,
		RepoClient:  repoClient,
		HTTPClient:  nil,
		Owner:       owner,
		Repo:        repo,
		GraphClient: nil,
		Dlogger:     &l,
	}
	// TODO, likely this should be a "scorecard" policy that runs multiple checks
	// here, and uses config to enable/disable checks.
	res := checks.BinaryArtifacts(cr)
	if res.Error2 != nil {
		return nil, res.Error2
	}

	var notify string
	if res.Score < checker.MaxResultScore {
		notify = fmt.Sprintf("Scorecard Check Binary Artifacts failed: %v\n"+
			"Please run scorecard directly for details: https://github.com/ossf/scorecard\n",
			res.Reason)
	}

	return &policydef.Result{
		Enabled:    enabled,
		Pass:       res.Score >= checker.MaxResultScore,
		NotifyText: notify,
		Details: details{
			Messages: l.Messages2,
		},
	}, nil
}

// Fix implementing policydef.Policy.Fix(). Scorecard checks will not have a Fix option.
func (b Binary) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	log.Warn().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Msg("Action fix is configured, but not implemented.")
	return nil
}

// GetAction returns the configured action from this policy's configuration
// stored in the org-level repo, default log. Implementing
// policydef.Policy.GetAction()
func (b Binary) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, rc, repo)
	return mc.Action
}

func (b Binary) GetOrgActionConfig(ctx context.Context, c *github.Client, owner, repo string) configdef.OrgActionConfig {
	return getOrgActionConfig(ctx, c, owner, repo)
}

func getOrgActionConfig(ctx context.Context, c *github.Client, owner, repo string) configdef.OrgActionConfig {
	oc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, rc, repo)
	return mc.ActionConfig
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig) {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action: "log",
	}
	if err := configFetchConfig(ctx, c, owner, operator.OrgConfigRepo, configFile, oc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", operator.OrgConfigRepo).
			Str("area", polName).
			Str("file", configFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	rc := &RepoConfig{}
	if err := configFetchConfig(ctx, c, owner, repo, path.Join(operator.RepoConfigDir, configFile), rc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Str("area", polName).
			Str("file", path.Join(operator.RepoConfigDir, configFile)).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	return oc, rc
}

func mergeConfig(oc *OrgConfig, rc *RepoConfig, repo string) *mergedConfig {
	mc := &mergedConfig{
		Action: oc.Action,
		ActionConfig: configdef.OrgActionConfig{
			IssueLabel: operator.GitHubIssueLabel,
			IssueFooter: operator.GitHubIssueFooter,
		},
	}

	if !oc.OptConfig.DisableRepoOverride {
		if rc.Action != nil {
			mc.Action = *rc.Action
		}
	}

	if oc.OptConfig.DisableRepoOverride {
		if len(oc.ActionConfig.IssueLabel) > 0 {
			mc.ActionConfig.IssueLabel = oc.ActionConfig.IssueLabel
		}
		if len(oc.ActionConfig.IssueFooter) > 0 {
			mc.ActionConfig.IssueFooter = oc.ActionConfig.IssueFooter
		}
	} else {
		if len(rc.ActionConfig.IssueLabel) > 0 {
			mc.ActionConfig.IssueLabel = rc.ActionConfig.IssueLabel
		}
		if len(rc.ActionConfig.IssueFooter) > 0 {
			mc.ActionConfig.IssueFooter = rc.ActionConfig.IssueFooter
		}
	}	
	return mc
}
