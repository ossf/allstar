// Copyright 2023 Allstar Authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package codeowners implements the CODEOWNERS policy.
package codeowners

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v59/github"
	"github.com/rs/zerolog/log"
)

const configFile = "codeowners.yaml"
const polName = "CODEOWNERS"

const notifyText = `A CODEOWNERS file can give users information about who is responsible for the maintenance of the repository, or specific folders/files. This is different the access control/permissions on a repository.

To fix this, add a CODEOWNERS file to your repository, following the official Github documentation and maybe your company's policy.
https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners`

// OrgConfig is the org-level config definition for CODEOWNERS
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride applies to all
	// BP config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`

	// RequireCODEOWNERS : set to true to require presence of a CODEOWNERS on the repositories (creates an issue if not present)
	// default false (only checks if existing CODEOWNERS is valid, creates issues if not valid).
	RequireCODEOWNERS bool `json:"requireCODEOWNERS"`
}

// RepoConfig is the repo-level config for CODEOWNERS
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `json:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `json:"action"`

	// RequireCODEOWNERS : set to true to require presence of a CODEOWNERS on the repositories (creates an issue if not present)
	// default false (only checks if existing CODEOWNERS is valid, creates issues if not valid).
	RequireCODEOWNERS *bool `json:"requireCODEOWNERS"`
}

type repositories interface {
	GetCodeownersErrors(ctx context.Context, owner, repo string, op *github.GetCodeownersErrorsOptions) (*github.CodeownersErrors, *github.Response, error)
}

type mergedConfig struct {
	Action            string
	RequireCODEOWNERS bool
}

type details struct {
	CodeownersFound  bool
	ErrorCount       int
	CodeownersErrors github.CodeownersErrors
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error

var configIsEnabled func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig, c *github.Client, owner, repo string) (bool, error)

func init() {
	configFetchConfig = config.FetchConfig
	configIsEnabled = config.IsEnabled
}

// Codeowners is the CODEOWNERS policy object, implements policydef.Policy.
type Codeowners bool

// NewCodeowners returns a new CODEOWNERS policy.
func NewCodeowners() policydef.Policy {
	var s Codeowners
	return s
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (s Codeowners) Name() string {
	return polName
}

// Check performs the policy check for CODEOWNERS policy based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (s Codeowners) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	return check(ctx, c.Repositories, c, owner, repo)
}

// Check whether this policy is enabled or not
func (s Codeowners) IsEnabled(ctx context.Context, c *github.Client, owner, repo string) (bool, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	return configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
}

func check(ctx context.Context, rep repositories, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	enabled, err := configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)

	if err != nil {
		return nil, err
	}

	var d details

	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Bool("enabled", enabled).
		Msg("Check repo enabled")

	codeownererrors, resp, err := rep.GetCodeownersErrors(ctx, owner, repo, nil)

	if err == nil {
		// "CODEOWNERS" exists
		d.ErrorCount = len(codeownererrors.Errors)
		d.CodeownersFound = true
		// the CODEOWNERS is present and has no errors, pass
		if d.ErrorCount == 0 {
			return &policydef.Result{
				Enabled:    enabled,
				Pass:       true,
				NotifyText: "",
				Details:    d,
			}, nil
		}
		// otherwise, fail because CODEOWNERS exists and has errors
		d.CodeownersErrors = *codeownererrors
		var errorMessage = fmt.Sprintf("%s\nCODEOWNERS file present but has %d errors.\n", notifyText, d.ErrorCount)
		for _, e := range codeownererrors.Errors {
			errorMessage += fmt.Sprintf("- %s\n  - %s\n", e.Path, e.Message)
		}
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       false,
			NotifyText: errorMessage,
			Details:    d,
		}, nil

	} else if resp != nil && resp.StatusCode == http.StatusNotFound {
		// "CODEOWNERS" does not exist, err is also not nil but we don't need it

		// if we require CODEOWNERS on all repositories in the Org
		if oc.RequireCODEOWNERS {
			d.CodeownersFound = false
			return &policydef.Result{
				Enabled:    enabled,
				Pass:       false,
				NotifyText: "CODEOWNERS file not present.\n" + notifyText,
				Details:    d,
			}, nil
		}
		d.CodeownersFound = false
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       true,
			NotifyText: "CODEOWNERS file not present.\n" + notifyText,
			Details:    d,
		}, nil

	}
	// Unknown error getting "CODEOWNERS", this could be an HTTP 500
	return nil, err
}

// Fix implementing policydef.Policy.Fix(). Currently not supported. Plan
// to support this TODO.
func (s Codeowners) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	log.Warn().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Msg("Action fix is configured, but not implemented.")
	return nil
}

// GetAction returns the configured action from CODEOWNERS policy's
// configuration stored in the org-level repo, default log. Implementing
// policydef.Policy.GetAction()
func (s Codeowners) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, orc, rc, repo)
	return mc.Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig, *RepoConfig) {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action:            "log",
		RequireCODEOWNERS: false,
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
		Action:            oc.Action,
		RequireCODEOWNERS: oc.RequireCODEOWNERS,
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
	if rc.RequireCODEOWNERS != nil {
		mc.RequireCODEOWNERS = *rc.RequireCODEOWNERS
	}
	return mc
}
