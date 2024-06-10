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

// Package roadie implements the Roadie catalog-info.yaml check policy.
package catalog

import (
	"context"
	"fmt"

	"github.com/contentful/allstar/pkg/config"
	"github.com/contentful/allstar/pkg/policydef"
	"github.com/shurcooL/githubv4"

	"github.com/google/go-github/v50/github"
	"github.com/rs/zerolog/log"
)

const configFile = "catalog.yaml"
const polName = "Catalog"

const notifyText = `A catalog-info.yaml file can give users information about which team is responsible for the maintenance of the repository.

To fix this, add a catalog-info.yaml file to your repository, following the official documentation.
<add-confluence-link-here>`

// OrgConfig is the org-level config definition for Catalog policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride applies to all
	// BP config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`

	// RequireCatalog : set to true to require presence of a catalog-info.yaml on the repositories (creates an issue if not present)
	// default false (only checks if existing catalog-info.yaml is valid, creates issues if not valid).
	RequireCatalog bool `json:"requireCatalog"`
}

// RepoConfig is the repo-level config for catalog-info.yaml
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `json:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `json:"action"`

	// RequireCatalog : set to true to require presence of a catalog-info.yaml on the repositories (creates an issue if not present)
	// default false (only checks if existing catalog-info.yaml is valid, creates issues if not valid).
	RequireCatalog *bool `json:"requireCatalog"`
}

type v4client interface {
	Query(context.Context, interface{}, map[string]interface{}) error
}

type mergedConfig struct {
	Action         string
	RequireCatalog bool
}

type details struct {
	Enabled      bool
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error

var configIsEnabled func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig, c *github.Client, owner, repo string) (bool, error)

//var catalogExists func(ctx context.Context, c *github.Client, owner, repo string) (bool, error)

func init() {
	configFetchConfig = config.FetchConfig
	configIsEnabled = config.IsEnabled
}

// Catalog is the catalog-info.yaml policy object, implements policydef.Policy.
type Catalog bool

// NewCatalog returns a new catalog-info.yaml policy.
func NewCatalog() policydef.Policy {
	var s Catalog
	return s
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (s Catalog) Name() string {
	return polName
}

// Check performs the policy check for catalog-info.yaml policy based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (s Catalog) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	v4c := githubv4.NewClient(c.Client())
	return check(ctx, c, v4c, owner, repo)
}

// Check whether this policy is enabled or not
func (s Catalog) IsEnabled(ctx context.Context, c *github.Client, owner, repo string) (bool, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	return configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
}

func check(ctx context.Context, c *github.Client, v4c v4client, owner,
	repo string) (*policydef.Result, error) {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	enabled, err := configIsEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, c, owner, repo)
	if err != nil {
		return nil, err
	}
	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Bool("enabled", enabled).
		Msg("Check repo enabled")

	var q struct {
		Repository struct {
			Object struct {
				Blob struct {
					Text string
				} `graphql:"... on Blob"`
			} `graphql:"object(expression: $expression)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	variables := map[string]interface{}{
		"owner":      githubv4.String(owner),
		"name":       githubv4.String(repo),
		"expression": githubv4.String(fmt.Sprintf("%s:%s", "HEAD", "catalog-info.yaml")),
	}
	if err := v4c.Query(ctx, &q, variables); err != nil {
		return nil, err
	}
	if len(q.Repository.Object.Blob.Text) == 0 {
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       false,
			NotifyText: "catalog-info.yaml file not found.\n" + fmt.Sprint(notifyText, owner, repo),
			Details: details{
				Enabled: false,
			},
		}, nil
	}
	return &policydef.Result{
		Enabled:    enabled,
		Pass:       true,
		NotifyText: "",
		Details: details{
			Enabled: true,
		},
	}, nil
}

// Fix implementing policydef.Policy.Fix(). Currently not supported. Plan
// to support this TODO.
func (s Catalog) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	log.Warn().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Msg("Action fix is configured, but not implemented.")
	return nil
}

// GetAction returns the configured action from catalog-info.yaml policy's
// configuration stored in the org-level repo, default log. Implementing
// policydef.Policy.GetAction()
func (s Catalog) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, orc, rc, repo)
	return mc.Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig, *RepoConfig) {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action:         "log",
		RequireCatalog: false,
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
		Action:         oc.Action,
		RequireCatalog: oc.RequireCatalog,
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
	if rc.RequireCatalog != nil {
		mc.RequireCatalog = *rc.RequireCatalog
	}
	return mc
}
