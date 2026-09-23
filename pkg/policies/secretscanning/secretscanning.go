// Copyright 2026 Allstar Authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package secretscanning implements the GitHub secret scanning policy.
package secretscanning

import (
	"context"
	"fmt"

	"github.com/google/go-github/v84/github"
	"github.com/rs/zerolog/log"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
)

const (
	configFile = "secret_scanning.yaml"
	polName    = "Secret Scanning"
)

const notifyText = `GitHub secret scanning looks for supported secret formats committed to a repository and creates alerts when it finds them.

To fix this, enable secret scanning in the repository's security settings. Go to https://github.com/%v/%v/settings/security_analysis to enable it.

For more information, see https://docs.github.com/en/code-security/secret-scanning/about-secret-scanning.`

// OrgConfig is the org-level config definition for the Secret Scanning policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue.
	Action string `json:"action"`
}

// RepoConfig is the repo-level config for the Secret Scanning policy.
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
	Status    string
	Available bool
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error

var configIsEnabled func(context.Context, config.OrgOptConfig, config.RepoOptConfig, config.RepoOptConfig, *github.Client, string, string) (bool, error)

func init() {
	configFetchConfig = config.FetchConfig
	configIsEnabled = config.IsEnabled
}

// SecretScanning implements policydef.Policy.
type SecretScanning bool

// NewSecretScanning returns a new GitHub secret scanning policy.
func NewSecretScanning() policydef.Policy {
	var s SecretScanning
	return s
}

// Name returns the human-readable policy name.
func (s SecretScanning) Name() string {
	return polName
}

type repositories interface {
	Get(context.Context, string, string) (*github.Repository, *github.Response, error)
	Edit(context.Context, string, string, *github.Repository) (*github.Repository, *github.Response, error)
}

// Check checks whether GitHub reports secret scanning as enabled for a repository.
func (s SecretScanning) Check(ctx context.Context, c *github.Client, owner, repo string) (*policydef.Result, error) {
	return check(ctx, c.Repositories, c, owner, repo)
}

// IsEnabled checks whether the policy is enabled by its configuration.
func (s SecretScanning) IsEnabled(ctx context.Context, c *github.Client, owner, repo string) (bool, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	return configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
}

func check(ctx context.Context, rep repositories, c *github.Client, owner, repo string) (*policydef.Result, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	enabled, err := configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
	if err != nil {
		return nil, err
	}

	repository, _, err := rep.Get(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	status, available := secretScanningStatus(repository)
	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Bool("enabled", enabled).
		Str("status", status).
		Bool("available", available).
		Msg("Checking secret scanning policy")

	result := &policydef.Result{
		Enabled: enabled,
		Pass:    true,
		Details: details{
			Status:    status,
			Available: available,
		},
	}
	if !available {
		return result, nil
	}
	if status == "enabled" {
		return result, nil
	}
	result.Pass = false
	result.NotifyText = "Secret scanning not enabled.\n" + fmt.Sprintf(notifyText, owner, repo)
	return result, nil
}

func secretScanningStatus(repository *github.Repository) (string, bool) {
	if repository == nil || repository.SecurityAndAnalysis == nil || repository.SecurityAndAnalysis.SecretScanning == nil {
		return "unavailable", false
	}
	return repository.SecurityAndAnalysis.SecretScanning.GetStatus(), true
}

// Fix enables GitHub secret scanning for the repository.
func (s SecretScanning) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	return fix(ctx, c.Repositories, owner, repo)
}

func fix(ctx context.Context, rep repositories, owner, repo string) error {
	_, _, err := rep.Edit(ctx, owner, repo, &github.Repository{
		SecurityAndAnalysis: &github.SecurityAndAnalysis{
			SecretScanning: &github.SecretScanning{Status: github.Ptr("enabled")},
		},
	})
	return err
}

// GetAction returns the configured action for this policy.
func (s SecretScanning) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	return mergeConfig(oc, orc, rc).Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig, *RepoConfig) {
	oc := &OrgConfig{Action: "log"}
	if err := configFetchConfig(ctx, c, owner, "", configFile, config.OrgLevel, oc); err != nil {
		log.Error().Err(err).Str("org", owner).Str("repo", repo).Str("area", polName).Msg("Using default org config")
	}
	orc := &RepoConfig{}
	if err := configFetchConfig(ctx, c, owner, repo, configFile, config.OrgRepoLevel, orc); err != nil {
		log.Error().Err(err).Str("org", owner).Str("repo", repo).Str("area", polName).Msg("Using default org repo config")
	}
	rc := &RepoConfig{}
	if err := configFetchConfig(ctx, c, owner, repo, configFile, config.RepoLevel, rc); err != nil {
		log.Error().Err(err).Str("org", owner).Str("repo", repo).Str("area", polName).Msg("Using default repo config")
	}
	return oc, orc, rc
}

func mergeConfig(oc *OrgConfig, orc, rc *RepoConfig) *mergedConfig {
	merged := &mergedConfig{Action: oc.Action}
	if orc.Action != nil {
		merged.Action = *orc.Action
	}
	if !oc.OptConfig.DisableRepoOverride && rc.Action != nil {
		merged.Action = *rc.Action
	}
	return merged
}
