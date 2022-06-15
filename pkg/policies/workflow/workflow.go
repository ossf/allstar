// Copyright 2022 Allstar Authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package workflow implements the Dangerous Workflow security policy check
// from scorecard.
package workflow

import (
	"context"
	"fmt"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/policydef"
	"github.com/ossf/allstar/pkg/scorecard"
	"github.com/ossf/scorecard/v4/checker"
	"github.com/ossf/scorecard/v4/checks"

	"github.com/google/go-github/v43/github"
	"github.com/rs/zerolog/log"
)

var doNothingOnOptOut = operator.DoNothingOnOptOut

const configFile = "dangerous_workflow.yaml"
const polName = "Dangerous Workflow"

// OrgConfig is the org-level config definition for this policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride applies to all
	// config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`
}

// RepoConfig is the repo-level config for this policy.
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `json:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `json:"action"`
}

type mergedConfig struct {
	Action string
}

type details struct {
	Findings []string
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error

func init() {
	configFetchConfig = config.FetchConfig
}

// Workflow is the Dangerous Workflow policy object, implements
// policydef.Policy.
type Workflow bool

// NewWorkflow returns a new Dangerous Workflow policy.
func NewWorkflow() policydef.Policy {
	var b Workflow
	return b
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (b Workflow) Name() string {
	return polName
}

// Check performs the policy check for this policy based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (b Workflow) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	enabled, err := config.IsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
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
			Details:    details{},
		}, nil
	}

	fullName := fmt.Sprintf("%s/%s", owner, repo)
	tr := c.Client().Transport
	scc, err := scorecard.Get(ctx, fullName, tr)
	if err != nil {
		return nil, err
	}

	l := checker.NewLogger()
	cr := &checker.CheckRequest{
		Ctx:        ctx,
		RepoClient: scc.ScRepoClient,
		Repo:       scc.ScRepo,
		Dlogger:    l,
	}

	res := checks.DangerousWorkflow(cr)
	if res.Error != nil {
		return nil, res.Error
	}

	logs := convertLogs(l.Flush())
	pass := res.Score >= checker.MaxResultScore
	var notify string
	if !pass {
		notify = fmt.Sprintf(`Project is out of compliance with Dangerous Workflow policy: %v

**Rule Description**
Dangerous Workflows are GitHub Action workflows that exhibit dangerous patterns that could render them vulnerable to attack. A vulnerable workflow is susceptible to leaking repository secrets, or allowing an attacker write access using the GITHUB_TOKEN. For more information about the particular patterns that are detected see the [Security Scorecards Documentation](https://github.com/ossf/scorecard/blob/main/docs/checks.md#dangerous-workflow) for Dangerous Workflow.

**Remediation Steps**
Avoid the dangerous workflow patterns. See this [post](https://securitylab.github.com/research/github-actions-preventing-pwn-requests/) for information on avoiding untrusted code checkouts. See this [document](https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections) for information on avoiding and mitigating the risk of script injections.


`,
			res.Reason)
		if len(logs) > 10 {
			notify += fmt.Sprintf(
				"**First 10 Dangerous Patterns Found**\n\n%v"+
					"- Run a Scorecards scan to see full list.\n\n",
				listJoin(logs[:10]))
		} else {
			notify += fmt.Sprintf("**Dangerous Patterns Found**\n\n%v\n", listJoin(logs))
		}
		notify += `**Additional Information**
This policy is drawn from [Security Scorecards](https://github.com/ossf/scorecard/), which is a tool that scores a project's adherence to security best practices. You may wish to run a Scorecards scan directly on this repository for more details.`
	}

	return &policydef.Result{
		Enabled:    enabled,
		Pass:       pass,
		NotifyText: notify,
		Details: details{
			Findings: logs,
		},
	}, nil
}

func listJoin(list []string) string {
	var s string
	for _, l := range list {
		s += fmt.Sprintf("- %v\n", l)
	}
	return s
}

func convertLogs(logs []checker.CheckDetail) []string {
	var s []string
	for _, l := range logs {
		s = append(s, fmt.Sprintf("%v[%v]:%v", l.Msg.Path, l.Msg.Offset, l.Msg.Text))
	}
	return s
}

// Fix implementing policydef.Policy.Fix(). Scorecard checks will not have a Fix option.
func (b Workflow) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
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
func (b Workflow) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, orc, rc, repo)
	return mc.Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig, *RepoConfig) {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action: "log",
	}
	if err := configFetchConfig(ctx, c, owner, "", configFile, config.OrgLevel, oc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Bool("orgLevel", true).
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
			Bool("orgLevel", false).
			Str("area", polName).
			Str("file", configFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	return oc, orc, rc
}

func mergeConfig(oc *OrgConfig, orc, rc *RepoConfig, repo string) *mergedConfig {
	mc := &mergedConfig{
		Action: oc.Action,
	}
	mc = mergeInRepoConfig(mc, orc, repo)

	if !oc.OptConfig.DisableRepoOverride {
		mc = mergeInRepoConfig(mc, rc, repo)
	}
	return mc
}

func mergeInRepoConfig(mc *mergedConfig, rc *RepoConfig, repo string) *mergedConfig {
	if rc.Action != nil {
		mc.Action = *rc.Action
	}
	return mc
}
