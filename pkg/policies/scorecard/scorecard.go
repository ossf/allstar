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

// Package scorecard implements the generic Security Scorecards policy
package scorecard

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/policydef"
	"github.com/ossf/allstar/pkg/scorecard"
	"github.com/ossf/scorecard/v4/checker"
	"github.com/ossf/scorecard/v4/checks"

	"github.com/google/go-github/v43/github"
	"github.com/rs/zerolog/log"
)

const configFile = "scorecard.yaml"
const polName = "Security Scorecards"

// OrgConfig is the org-level config definition for this policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride
	// applies to all config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`

	// Checks is a list of check names to run from Security Scorecards. These
	// must match the name that the check uses in it's call to
	// "registerCheck". See the check code for each name:
	// https://github.com/ossf/scorecard/tree/main/checks For example, the name
	// for the Signed Releases check is "Signed-Releases".
	Checks []string `json:"checks"`

	// Threshold is the score threshold that checks must meet to pass the
	// policy. If all checks score equal or above the threshold, the Allstar
	// policy will pass. The default is checker.MaxResultScore:
	// https://pkg.go.dev/github.com/ossf/scorecard/v4@v4.4.0/checker#pkg-constants
	Threshold int `json:"threshold"`
}

// RepoConfig is the repo-level config for this policy.
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `json:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `json:"action"`

	// Checks overrides the same setting in org-level, only if present.
	Checks *[]string `json:"checks"`

	// Threshold overrides the same setting in org-level, only if present.
	Threshold *int `json:"threshold"`
}

type mergedConfig struct {
	Action    string
	Checks    []string
	Threshold int
}

type details struct {
	// Findings key is the check name, and value are logs from Scorecards.
	Findings map[string][]string
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error
var configIsEnabled func(context.Context, config.OrgOptConfig, config.RepoOptConfig, config.RepoOptConfig, *github.Client, string, string) (bool, error)
var scorecardGet func(context.Context, string, http.RoundTripper) (*scorecard.ScClient, error)

var doNothingOnOptOut = operator.DoNothingOnOptOut

var checksAllChecks checker.CheckNameToFnMap

func init() {
	configFetchConfig = config.FetchConfig
	configIsEnabled = config.IsEnabled
	checksAllChecks = checks.AllChecks
	scorecardGet = scorecard.Get
}

// Scorecard is the Security Scorecard policy object, implements
// policydef.Policy.
type Scorecard bool

// NewScorecard returns a new Scorecard policy.
func NewScorecard() policydef.Policy {
	var b Scorecard
	return b
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (b Scorecard) Name() string {
	return polName
}

// Check performs the policy check for this policy based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (b Scorecard) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	enabled, err := configIsEnabled(ctx, oc.OptConfig, orc.OptConfig,
		rc.OptConfig, c, owner, repo)
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
	mc := mergeConfig(oc, orc, rc, repo)

	fullName := fmt.Sprintf("%s/%s", owner, repo)
	tr := c.Client().Transport
	scc, err := scorecardGet(ctx, fullName, tr)
	if err != nil {
		return nil, err
	}

	var notify string
	pass := true
	f := make(map[string][]string)

	for _, n := range mc.Checks {

		l := checker.NewLogger()
		cr := &checker.CheckRequest{
			Ctx:        ctx,
			RepoClient: scc.ScRepoClient,
			Repo:       scc.ScRepo,
			Dlogger:    l,
		}

		res := checksAllChecks[n].Fn(cr)
		if res.Error != nil {
			// We are not sure that all checks are safe to run inside Allstar, some
			// might error, and we don't want to abort a whole org enforcement loop
			// for an expected error. Just log the error and move on. If there are
			// any results from a previous check, those can be returned, so just
			// break from the loop here.
			log.Warn().
				Str("org", owner).
				Str("repo", repo).
				Str("area", polName).
				Str("check", n).
				Err(res.Error).
				Msg("Scorecard check errored.")
			break
		}

		logs := convertLogs(l.Flush())
		if len(logs) > 0 {
			f[n] = logs
		}
		if res.Score < mc.Threshold && res.Score != checker.InconclusiveResultScore {
			pass = false
			if notify == "" {
				notify = `Project is out of compliance with Security Scorecards policy

**Rule Description**
This is a generic passthrough policy that runs the configured checks from Security Scorecards. Please see the [Security Scorecards Documentation](https://github.com/ossf/scorecard/blob/main/docs/checks.md#dangerous-workflow) for more infomation on each check.

`
			}
			if len(logs) > 10 {
				notify += fmt.Sprintf(
					"**First 10 Results from policy: %v : %v **\n\n%v"+
						"- Run a Scorecards scan to see full list.\n\n",
					res.Name, res.Reason, listJoin(logs[:10]))
			} else {
				notify += fmt.Sprintf("**Results from policy: %v : %v **\n\n%v\n",
					res.Name, res.Reason, listJoin(logs))
			}
		}
	}

	return &policydef.Result{
		Enabled:    enabled,
		Pass:       pass,
		NotifyText: notify,
		Details: details{
			Findings: f,
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
func (b Scorecard) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
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
func (b Scorecard) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, orc, rc, repo)
	return mc.Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig, *RepoConfig) {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action:    "log",
		Threshold: checker.MaxResultScore,
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
		Action:    oc.Action,
		Checks:    oc.Checks,
		Threshold: oc.Threshold,
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
	if rc.Checks != nil {
		mc.Checks = *rc.Checks
	}
	if rc.Threshold != nil {
		mc.Threshold = *rc.Threshold
	}
	return mc
}
