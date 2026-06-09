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

// Package secretscanning implements the GitHub Secret Scanning security policy.
package secretscanning

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v84/github"
	"github.com/rs/zerolog/log"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
)

const (
	configFile        = "secret_scanning.yaml"
	polName           = "Secret Scanning"
	secretEnabled     = "enabled"
	secretDisabled    = "disabled"
	secretUnavailable = "unavailable"
)

const notifyText = `GitHub secret scanning checks repositories for known secret formats and alerts maintainers when credentials or tokens are detected.

To fix this, enable secret scanning in repository settings. Go to https://github.com/%v/%v/settings/security_analysis to enable.

For more information, see https://docs.github.com/en/code-security/secret-scanning/about-secret-scanning.`

// OrgConfig is the org-level config definition for Secret Scanning.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride applies to all
	// Secret Scanning config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`
}

// RepoConfig is the repo-level config for Secret Scanning.
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
	Available bool
	Status    string
	URL       string
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error

var configIsEnabled func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig, c *github.Client, owner, repo string) (bool, error)

func init() {
	configFetchConfig = config.FetchConfig
	configIsEnabled = config.IsEnabled
}

type repositories interface {
	Get(context.Context, string, string) (*github.Repository, *github.Response, error)
	Edit(context.Context, string, string, *github.Repository) (*github.Repository, *github.Response, error)
}

// SecretScanning is the Secret Scanning policy object, implements policydef.Policy.
type SecretScanning bool

// NewSecretScanning returns a new Secret Scanning policy.
func NewSecretScanning() policydef.Policy {
	var s SecretScanning
	return s
}

// Name returns the name of this policy, implementing policydef.Policy.Name().
func (s SecretScanning) Name() string {
	return polName
}

// Check performs the policy check for Secret Scanning based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check().
func (s SecretScanning) Check(ctx context.Context, c *github.Client, owner,
	repo string,
) (*policydef.Result, error) {
	return check(ctx, c.Repositories, c, owner, repo)
}

// Check whether this policy is enabled or not.
func (s SecretScanning) IsEnabled(ctx context.Context, c *github.Client, owner, repo string) (bool, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	return configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
}

func check(ctx context.Context, rep repositories, c *github.Client, owner,
	repo string,
) (*policydef.Result, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	enabled, err := configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
	if err != nil {
		return nil, err
	}

	r, rsp, err := rep.Get(ctx, owner, repo)
	if err != nil {
		if rsp != nil && rsp.StatusCode == http.StatusForbidden {
			log.Warn().
				Str("org", owner).
				Str("repo", repo).
				Str("area", polName).
				Err(err).
				Msg("Secret scanning status unavailable.")
			return unavailableResult(enabled, owner, repo), nil
		}
		return nil, err
	}

	status, available := secretScanningStatus(r)
	if !available {
		return unavailableResult(enabled, owner, repo), nil
	}

	if status == secretDisabled {
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       false,
			NotifyText: "Secret scanning not enabled.\n" + fmt.Sprintf(notifyText, owner, repo),
			Details: details{
				Available: true,
				Status:    status,
				URL:       securityAnalysisURL(owner, repo),
			},
		}, nil
	}

	return &policydef.Result{
		Enabled:    enabled,
		Pass:       true,
		NotifyText: "",
		Details: details{
			Available: true,
			Status:    status,
			URL:       securityAnalysisURL(owner, repo),
		},
	}, nil
}

func unavailableResult(enabled bool, owner, repo string) *policydef.Result {
	return &policydef.Result{
		Enabled:    enabled,
		Pass:       true,
		NotifyText: "",
		Details: details{
			Available: false,
			Status:    secretUnavailable,
			URL:       securityAnalysisURL(owner, repo),
		},
	}
}

func secretScanningStatus(r *github.Repository) (string, bool) {
	if r == nil ||
		r.SecurityAndAnalysis == nil ||
		r.SecurityAndAnalysis.SecretScanning == nil ||
		r.SecurityAndAnalysis.SecretScanning.Status == nil ||
		r.SecurityAndAnalysis.SecretScanning.GetStatus() == "" {
		return secretUnavailable, false
	}
	return r.SecurityAndAnalysis.SecretScanning.GetStatus(), true
}

func securityAnalysisURL(owner, repo string) string {
	return fmt.Sprintf("https://github.com/%v/%v/settings/security_analysis", owner, repo)
}

// Fix implementing policydef.Policy.Fix().
func (s SecretScanning) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	return fix(ctx, c.Repositories, c, owner, repo)
}

func fix(ctx context.Context, rep repositories, c *github.Client, owner, repo string) error {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	enabled, err := configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}

	r, rsp, err := rep.Get(ctx, owner, repo)
	if err != nil {
		if rsp != nil && rsp.StatusCode == http.StatusForbidden {
			log.Warn().
				Str("org", owner).
				Str("repo", repo).
				Str("area", polName).
				Err(err).
				Msg("Action fix is configured, but secret scanning status is unavailable.")
			return nil
		}
		return err
	}

	status, available := secretScanningStatus(r)
	if !available || status != secretDisabled {
		return nil
	}

	_, rsp, err = rep.Edit(ctx, owner, repo, &github.Repository{
		SecurityAndAnalysis: &github.SecurityAndAnalysis{
			SecretScanning: &github.SecretScanning{
				Status: github.Ptr(secretEnabled),
			},
		},
	})
	if err != nil {
		if rsp != nil && rsp.StatusCode == http.StatusForbidden {
			log.Warn().
				Str("org", owner).
				Str("repo", repo).
				Str("area", polName).
				Msg("Action fix is configured, but did not accept administration permissions update.")
			return nil
		}
		return err
	}
	return nil
}

// GetAction returns the configured action from Secret Scanning policy's
// configuration stored in the org-level repo, default log. Implementing
// policydef.Policy.GetAction().
func (s SecretScanning) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
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

func mergeConfig(oc *OrgConfig, orc *RepoConfig, rc *RepoConfig, repo string) *mergedConfig {
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
