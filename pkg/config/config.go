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

	// IssueLabel is the label used to tag, search, and identify GitHub Issues
	// created by the bot. The defeault is specified by the operator of Allstar,
	// currently: "allstar"
	IssueLabel string `yaml:"issueLabel"`

	// IssueRepo is the name of a repository in the organization to create issues
	// in. If left unset, by default Allstar will create issues in the repository
	// that is out of compliance. Setting the IssueRepo will instruct Allstar to
	// only create issues in the specified repository for non-compliance found in
	// any repository in the organization.
	//
	// This can be useful for previewing the issues that Allstar would create in
	// all repositories. Also, it can be used to centrally audit non-compliance
	// issues.
	//
	// Note: When changing this setting, Allstar does not clean up previously
	// created issues from a previous setting.
	IssueRepo string `yaml:"issueRepo"`

	// IssueFooter is a custom message to add to the end of all Allstar created
	// issues in the GitHub organization. It does not supercede the bot-level
	// footer (found in pkg/config/operator) but is added in addition to that
	// one. This setting is useful to direct users to the organization-level
	// config repository or documentation describing your Allstar settings and
	// policies.
	IssueFooter string `yaml:"issueFooter"`
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

	// IssueLabel is the label used to tag, search, and identify GitHub Issues
	// created by the bot. Repo-level label my override Org-level setting
	// regardless of Optconfig.DisableRepoOverride.
	IssueLabel string `yaml:"issueLabel"`
}

// RepoOptConfig is used in Allstar and policy-specific repo-level config to
// opt in/out of enforcement.
type RepoOptConfig struct {
	// OptIn : set to true to opt-in this repo when in opt-in strategy
	OptIn bool `yaml:"optIn"`

	// OptOut: set to true to opt-out this repo when in opt-out strategy
	OptOut bool `yaml:"optOut"`
}

const githubConfRepo = ".github"

// FetchConfig grabs a yaml config file from github and writes it to out.
func FetchConfig(ctx context.Context, c *github.Client, owner, repo, name string, orgLevel bool, out interface{}) error {
	return fetchConfig(ctx, c.Repositories, owner, repo, name, orgLevel, out)
}

func fetchConfig(ctx context.Context, r repositories, owner, repoIn, name string, orgLevel bool, out interface{}) error {
	var repo string
	var p string
	if orgLevel {
		_, rsp, err := r.Get(ctx, owner, operator.OrgConfigRepo)
		if err == nil {
			repo = operator.OrgConfigRepo
			p = name
		} else if rsp != nil && rsp.StatusCode == http.StatusNotFound {
			repo = githubConfRepo
			p = path.Join(operator.OrgConfigDir, name)
		} else {
			return err
		}
	} else {
		repo = repoIn
		p = path.Join(operator.RepoConfigDir, name)
	}
	cf, _, rsp, err := r.GetContents(ctx, owner, repo, p, nil)
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
			Str("file", p).
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
func IsEnabled(ctx context.Context, o OrgOptConfig, r RepoOptConfig, c *github.Client, owner, repo string) (bool, error) {
	return isEnabled(ctx, o, r, c.Repositories, owner, repo)
}

func isEnabled(ctx context.Context, o OrgOptConfig, r RepoOptConfig, rep repositories, owner, repo string) (bool, error) {
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
	oc, rc := getAppConfigs(ctx, r, owner, repo)
	enabled, err := isEnabled(ctx, oc.OptConfig, rc.OptConfig, r, owner, repo)
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

// GetAppConfigs gets the Allstar configurations for both Org and Repo level.
func GetAppConfigs(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig) {
	return getAppConfigs(ctx, c.Repositories, owner, repo)
}

func getAppConfigs(ctx context.Context, r repositories, owner, repo string) (*OrgConfig, *RepoConfig) {
	// drop errors, if cfg file is not there, go with defaults
	oc := &OrgConfig{}
	if err := fetchConfig(ctx, r, owner, "", operator.AppConfigFile, true, oc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Bool("orgLevel", true).
			Str("area", "bot").
			Str("file", operator.AppConfigFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	rc := &RepoConfig{}
	if err := fetchConfig(ctx, r, owner, repo, operator.AppConfigFile, false, rc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Bool("orgLevel", false).
			Str("area", "bot").
			Str("file", operator.AppConfigFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	return oc, rc
}

func contains(s []string, e string) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}
