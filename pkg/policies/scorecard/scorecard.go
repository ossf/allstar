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

// Package scorecard implements security policy checks from scorecard.
package scorecard

import (
	"context"
	"fmt"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
	"github.com/ossf/scorecard/v4/checker"
	"github.com/ossf/scorecard/v4/checks"
	"github.com/ossf/scorecard/v4/clients/githubrepo"

	"github.com/google/go-github/v39/github"
	"github.com/rs/zerolog/log"
)

const configFile = "scorecard.yaml"
const polName = "Scorecard"
const defaultGitRef = "HEAD"

// OrgConfig is the org-level config definition for this policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride applies to all
	// config.
	OptConfig config.OrgOptConfig `yaml:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `yaml:"action"`
}

// RepoConfig is the repo-level config for this policy.
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `yaml:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `yaml:"action"`
}

type mergedConfig struct {
	Action string
}

type details struct {
	Messages []checker.CheckDetail
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, bool, interface{}) error

func init() {
	configFetchConfig = config.FetchConfig
}

// Scorecard is the Scorecard Artifacts policy object, implements policydef.Policy.
type Scorecard bool

// NewScorecard returns a new Scorecard Artifacts policy.
func NewScorecard() policydef.Policy {
	var sc Scorecard
	return sc
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (sc Scorecard) Name() string {
	return polName
}

// Check performs the policy check for this policy based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (sc Scorecard) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	oc, rc := getConfig(ctx, c, owner, repo)
	enabled, err := config.IsEnabled(ctx, oc.OptConfig, rc.OptConfig, c, owner, repo)
	if err != nil {
		return nil, err
	}
	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Bool("enabled", enabled).
		Msg("Check repo enabled")
	if !enabled {
		// Don't run this policy unless enabled, as it is expensive. This is only
		// checking enablement of policy, but not Allstar overall, this is ok for
		// now.
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       true,
			NotifyText: "Disabled",
			Details:    details{},
		}, nil
	}

	scRepoArg := fmt.Sprintf("%s/%s", owner, repo)
	scRepo, err := githubrepo.MakeGithubRepo(scRepoArg)
	if err != nil {
		return nil, err
	}

	roundTripper := c.Client().Transport
	repoClient := githubrepo.CreateGithubRepoClientWithTransport(ctx, roundTripper)
	if err := repoClient.InitRepo(scRepo, defaultGitRef); err != nil {
		return nil, err
	}
	defer repoClient.Close()
	l := checker.NewLogger()
	cr := &checker.CheckRequest{
		Ctx:        ctx,
		RepoClient: repoClient,
		Repo:       scRepo,
		Dlogger:    l,
	}

	// TODO(scorecard): Likely this should be a "scorecard" policy that runs multiple checks
	// here, and uses config to enable/disable checks.
	res := checks.BinaryArtifacts(cr)
	if res.Error2 != nil {
		return nil, res.Error2
	}

	var notify string
	if res.Score < checker.MaxResultScore {
		notify = fmt.Sprintf("Scorecard Check Scorecard: %v\n"+
			"Please run scorecard directly for details: https://github.com/ossf/scorecard\n",
			res.Reason)
	}

	return &policydef.Result{
		Enabled:    enabled,
		Pass:       res.Score >= checker.MaxResultScore,
		NotifyText: notify,
		Details: details{
			Messages: l.Logs(),
		},
	}, nil
}

// Fix implementing policydef.Policy.Fix(). Scorecard checks will not have a Fix option.
func (sc Scorecard) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
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
func (sc Scorecard) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, rc, repo)
	return mc.Action
}

// TODO(policies): Consider de-duping config functions across policies
func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig) {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action: "log",
	}
	if err := configFetchConfig(ctx, c, owner, "", configFile, true, oc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Bool("orgLevel", true).
			Str("area", polName).
			Str("file", configFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	rc := &RepoConfig{}
	if err := configFetchConfig(ctx, c, owner, repo, configFile, false, rc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Bool("orgLevel", false).
			Str("area", polName).
			Str("file", configFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	return oc, rc
}

func mergeConfig(oc *OrgConfig, rc *RepoConfig, repo string) *mergedConfig {
	mc := &mergedConfig{
		Action: oc.Action,
	}

	if !oc.OptConfig.DisableRepoOverride {
		if rc.Action != nil {
			mc.Action = *rc.Action
		}
	}
	return mc
}
