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

// Package security implements the SECURITY.md security policy.
package security

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v35/github"
	"github.com/rs/zerolog/log"
)

const configFile = "security.yaml"
const polName = "SECURITY.md"
const filePath = "SECURITY.md"

// OrgConfig is the org-level config definition for Branch Protection.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride applies to all
	// BP config.
	OptConfig config.OrgOptConfig `yaml:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `yaml:"action"`

	//TODO add default contents for "fix" action
}

// RepoConfig is the repo-level config for Branch Protection
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
	Exists bool
	Empty  bool
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, interface{}) error

func init() {
	configFetchConfig = config.FetchConfig
}

// Security is the SECURITY.md policy object, implements policydef.Policy.
type Security bool

// NewSecurity returns a new SECURITY.md policy.
func NewSecurity() policydef.Policy {
	var s Security
	return s
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (s Security) Name() string {
	return polName
}

type repositories interface {
	GetContents(context.Context, string, string, string,
		*github.RepositoryContentGetOptions) (*github.RepositoryContent,
		[]*github.RepositoryContent, *github.Response, error)
}

// Check performs the polcy check for SECURITY.md policy based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (s Security) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	return check(ctx, c.Repositories, c, owner, repo)
}

func check(ctx context.Context, rep repositories, c *github.Client, owner,
	repo string) (*policydef.Result, error) {

	oc, rc := getConfig(ctx, c, owner, repo)
	enabled := config.IsEnabled(oc.OptConfig, rc.OptConfig, repo)
	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Bool("enabled", enabled).
		Msg("Check repo enabled")

	fc, _, rsp, err := rep.GetContents(ctx, owner, repo, filePath, nil)
	if err != nil {
		if rsp != nil && rsp.StatusCode == http.StatusNotFound {
			return &policydef.Result{
				Enabled: enabled,
				Pass:    false,
				NotifyText: fmt.Sprintf("SECURITY.md not found.\n"+
					"Go to https://github.com/%v/%v/security/policy to enable.\n", owner, repo),
				Details: details{
					Exists: false,
					Empty:  true,
				},
			}, nil
		}
		return nil, err
	}
	if fc.GetSize() <= 4 { // Empty file could have a carriage return, etc.
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       false,
			NotifyText: "SECURITY.md is empty.\n",
			Details: details{
				Exists: true,
				Empty:  true,
			},
		}, nil
	}
	return &policydef.Result{
		Enabled:    enabled,
		Pass:       true,
		NotifyText: "",
		Details: details{
			Exists: true,
			Empty:  false,
		},
	}, nil
}

// Fix implementing policydef.Policy.Fix(). Currently not supported. Plan
// to support this TODO.
func (s Security) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	log.Warn().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Msg("Action fix is configured, but not implemented.")
	return nil
}

// GetAction returns the configured action from SECURITY.md policy's
// configuration stored in the org-level repo, default log. Implementing
// policydef.Policy.GetAction()
func (s Security) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, rc, repo)
	return mc.Action
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
	}

	if !oc.OptConfig.DisableRepoOverride {
		if rc.Action != nil {
			mc.Action = *rc.Action
		}
	}
	return mc
}
