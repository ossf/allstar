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
	"path/filepath"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
	"github.com/ossf/allstar/pkg/scorecard"
	"github.com/ossf/scorecard/v4/checker"
	"github.com/ossf/scorecard/v4/checks"

	"github.com/google/go-github/v43/github"
	"github.com/rs/zerolog/log"
)

const configFile = "binary_artifacts.yaml"
const polName = "Binary Artifacts"

// OrgConfig is the org-level config definition for this policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride applies to all
	// config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`

	// IgnoreFiles is a list of file names to ignore. Any Binary Artifacts found
	// with these names are allowed, and the policy may still pass. These are
	// just the file name, not a full path. Globs are not allowed.
	IgnoreFiles []string `json:"ignoreFiles"`
}

// RepoConfig is the repo-level config for this policy.
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `json:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `json:"action"`

	// IgnorePaths is a list of full paths to ignore. If these are reported as a
	// Binary Artifact, they will be ignored and the policy may still pass. These
	// must be full paths with directories. Globs are not allowed. These are
	// allowed even if RepoOverride is false.
	IgnorePaths []string `json:"ignorePaths"`
}

type mergedConfig struct {
	Action      string
	IgnoreFiles []string
	IgnorePaths []string
}

type details struct {
	Artifacts []string
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error

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

// Check performs the policy check for this policy based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (b Binary) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, orc, rc, repo)
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

	res := checks.BinaryArtifacts(cr)
	if res.Error != nil {
		return nil, res.Error
	}

	logs := convertAndFilterLogs(l.Flush(), mc)

	// We assume every log is a finding and do filtering on the Allstar side
	pass := len(logs) == 0

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

func convertAndFilterLogs(logs []checker.CheckDetail, mc *mergedConfig) []string {
	var s []string
	for _, l := range logs {
		if in(l.Msg.Path, mc.IgnorePaths) {
			continue
		}
		if in(filepath.Base(l.Msg.Path), mc.IgnoreFiles) {
			continue
		}
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
		Action:      oc.Action,
		IgnoreFiles: oc.IgnoreFiles,
		IgnorePaths: rc.IgnorePaths,
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

func in(s string, l []string) bool {
	for _, v := range l {
		if s == v {
			return true
		}
	}
	return false
}
