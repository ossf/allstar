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

// Package config defines and grabs overall bot config.
package config

import (
	"context"
	"net/http"
	"path"

	"github.com/ossf/allstar/pkg/config/operator"

	"github.com/google/go-github/v39/github"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

// OrgConfig is the org-level config definition for Allstar
type OrgConfig struct {
	// OptConfig contains the opt in/out configuration.
	OptConfig OrgOptConfig `yaml:"optConfig"`
}

// OrgOptConfig is used in Allstar and policy-secific org-level config to
// define the opt in/out configuration.
type OrgOptConfig struct {
	// OptOutStrategy : set to true to change from opt-in to opt-out.
	OptOutStrategy bool `yaml:"optOutStrategy"`

	// OptInRepos is the list of repos to opt-in when in opt-in strategy.
	OptInRepos []string `yaml:"optInRepos"`

	// OptOutRepos is the list of repos to opt-out when in opt-out strategy.
	OptOutRepos []string `yaml:"optOutRepos"`

	// OptOutPrivateRepos : set to true to not access private repos.
	OptOutPrivateRepos bool `yaml:"optOutPrivateRepos"`

	// OptOutPublicRepos : set to true to not access public repos.
	OptOutPublicRepos bool `yaml:"optOutPublicRepos"`

	// DisableRepoOverride : set to true to disallow repos from opt-in/out in
	// their config.
	DisableRepoOverride bool `yaml:"disableRepoOverride"`
}

// RepoConfig is the repo-level config definition for Allstar
type RepoConfig struct {
	// OptConfig contains the opt in/out configuration.
	OptConfig RepoOptConfig `yaml:"optConfig"`
}

// RepoOptConfig is used in Allstar and policy-specific repo-level config to
// opt in/out of enforcement.
type RepoOptConfig struct {
	// OptIn : set to true to opt-in this repo when in opt-in strategy
	OptIn bool `yaml:"optIn"`

	// OptOut: set to true to opt-out this repo when in opt-out strategy
	OptOut bool `yaml:"optOut"`
}

// FetchConfig grabs a yaml config file from github and writes it to out.
func FetchConfig(ctx context.Context, c *github.Client, owner, repo, path string, out interface{}) error {
	return fetchConfig(ctx, c.Repositories, owner, repo, path, out)
}

func fetchConfig(ctx context.Context, r repositories, owner, repo, path string, out interface{}) error {
	cf, _, rsp, err := r.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		if rsp != nil && rsp.StatusCode == http.StatusNotFound {
			return nil
		}
		return err
	}
	con, err := cf.GetContent()
	if err != nil {
		return err
	}
	if err := yaml.UnmarshalStrict([]byte(con), out); err != nil {
		log.Warn().
			Str("org", owner).
			Str("repo", repo).
			Str("file", path).
			Err(err).
			Msg("Malformed config file, using defaults.")
		// TODO: if UnmarshalStrict errors, does it still fill out the found fields?
		return nil
	}
	return nil
}

type repositories interface {
	Get(context.Context, string, string) (*github.Repository,
		*github.Response, error)
	GetContents(context.Context, string, string, string,
		*github.RepositoryContentGetOptions) (*github.RepositoryContent,
		[]*github.RepositoryContent, *github.Response, error)
}

// IsEnabled determines if a repo is enabled by interpreting the provided
// org-level and repo-level OptConfigs.
func IsEnabled(ctx context.Context, o OrgOptConfig, r RepoOptConfig, rep repositories, owner, repo string) (bool, error) {
	var enabled bool

	gr, _, err := rep.Get(ctx, owner, repo)
	if err != nil {
		return false, err
	}

	if o.OptOutStrategy {
		enabled = true
		if contains(o.OptOutRepos, repo) {
			enabled = false
		}
		if o.OptOutPrivateRepos && gr.GetPrivate() {
			enabled = false
		}
		if o.OptOutPublicRepos && !gr.GetPrivate() {
			enabled = false
		}
		if !o.DisableRepoOverride && r.OptOut {
			enabled = false
		}
	} else {
		enabled = false
		if contains(o.OptInRepos, repo) {
			enabled = true
		}
		if !o.DisableRepoOverride && r.OptIn {
			enabled = true
		}
	}
	return enabled, nil
}

// IsBotEnabled determines if allstar is enabled overall on the provided repo.
func IsBotEnabled(ctx context.Context, c *github.Client, owner, repo string) bool {
	return isBotEnabled(ctx, c.Repositories, owner, repo)
}

func isBotEnabled(ctx context.Context, r repositories, owner, repo string) bool {
	// drop errors, if cfg file is not there, go with defaults
	oc := &OrgConfig{}
	if err := fetchConfig(ctx, r, owner, operator.OrgConfigRepo, operator.AppConfigFile, oc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", operator.OrgConfigRepo).
			Str("area", "bot").
			Str("file", operator.AppConfigFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	rc := &RepoConfig{}
	if err := fetchConfig(ctx, r, owner, repo, path.Join(operator.RepoConfigDir, operator.AppConfigFile), rc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Str("area", "bot").
			Str("file", path.Join(operator.RepoConfigDir, operator.AppConfigFile)).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}

	enabled, err := IsEnabled(ctx, oc.OptConfig, rc.OptConfig, r, owner, repo)
	if err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Str("owner", owner).
			Str("area", "bot").
			Bool("enabled", enabled).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", "bot").
		Bool("enabled", enabled).
		Msg("Check repo enabled")
	return enabled
}

func contains(s []string, e string) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}
