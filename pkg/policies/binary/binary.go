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

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
	"github.com/ossf/scorecard/v4/checker"
	"github.com/ossf/scorecard/v4/checks"
	"github.com/ossf/scorecard/v4/clients/githubrepo"

	"github.com/google/go-github/v43/github"
	"github.com/rs/zerolog/log"
)

const configFile = "binary_artifacts.yaml"
const polName = "Binary Artifacts"
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
	Artifacts []string
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, bool, interface{}) error

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

// TODO(log): Replace once scorecard supports a constructor for new loggers.
//            This is a copy of the `DetailLogger` implementation at:
//            https://github.com/ossf/scorecard/blob/ba503c3bee014d97c38f3f5caaeb6977935a9272/checker/detail_logger_impl.go
type logger struct {
	logs []checker.CheckDetail
}

func (l *logger) Info(msg *checker.LogMessage) {
	cd := checker.CheckDetail{
		Type: checker.DetailInfo,
		Msg:  *msg,
	}
	l.logs = append(l.logs, cd)
}

func (l *logger) Warn(msg *checker.LogMessage) {
	cd := checker.CheckDetail{
		Type: checker.DetailWarn,
		Msg:  *msg,
	}
	l.logs = append(l.logs, cd)
}

func (l *logger) Debug(msg *checker.LogMessage) {
	cd := checker.CheckDetail{
		Type: checker.DetailDebug,
		Msg:  *msg,
	}
	l.logs = append(l.logs, cd)
}

func (l *logger) Flush() []checker.CheckDetail {
	ret := l.logs
	l.logs = nil
	return ret
}

// Check performs the policy check for this policy based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (b Binary) Check(ctx context.Context, c *github.Client, owner,
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
	l := logger{}
	cr := &checker.CheckRequest{
		Ctx:        ctx,
		RepoClient: repoClient,
		Repo:       scRepo,
		Dlogger:    &l,
	}

	// TODO, likely this should be a "scorecard" policy that runs multiple checks
	// here, and uses config to enable/disable checks.
	res := checks.BinaryArtifacts(cr)
	if res.Error2 != nil {
		return nil, res.Error2
	}

	logs := convertLogs(l.logs)
	pass := res.Score >= checker.MaxResultScore
	var notify string
	if !pass {
		notify = fmt.Sprintf(`Project is out of compliance with Binary Artifacts policy: %v

**Rule Description**
Binary Artifacts are an increased security risk in your repository. Binary artifacts cannot be reviewed, allowing the introduction of possibly obsolete or maliciously subverted executables. For more information see the [Security Scorecards Documentation](https://github.com/ossf/scorecard/blob/main/docs/checks.md#binary-artifacts) for Binary Artifacts.

**Remediation Steps**
To remediate, remove the generated executable artifacts from the repository.

`,
			res.Reason)
		if len(logs) > 10 {
			notify += fmt.Sprintf(
				"**First 10 Artifacts Found**\n\n%v"+
					"- Run a Scorecards scan to see full list.\n\n",
				listJoin(logs[:10]))
		} else {
			notify += fmt.Sprintf("**Artifacts Found**\n\n%v\n", listJoin(logs))
		}
		notify += `**Additional Information**
This policy is drawn from [Security Scorecards](https://github.com/ossf/scorecard/), which is a tool that scores a project's adherence to security best practices. You may wish to run a Scorecards scan directly on this repository for more details.`
	}

	return &policydef.Result{
		Enabled:    enabled,
		Pass:       pass,
		NotifyText: notify,
		Details: details{
			Artifacts: logs,
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
		s = append(s, l.Msg.Path)
	}
	return s
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
