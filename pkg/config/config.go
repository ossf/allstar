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

// package config defines and grabs overall bot config
package config

import (
	"context"
	"log"
	"path"

	"github.com/google/go-github/v35/github"
	"gopkg.in/yaml.v2"
)

const config_OrgConfigRepo = ".allstar"
const config_RepoConfigDir = ".allstar"
const config_ConfigFile = "allstar.yaml"

type OrgConfig struct {
	OptConfig OrgOptConfig `yaml:"optConfig"`
}

type OrgOptConfig struct {
	OptOutStrategy      bool     `yaml:"optOutStrategy"`      // Set to true to change from opt-in to opt-out
	OptInRepos          []string `yaml:"optInRepos"`          // List of repos to opt-in when in opt-in strategy
	OptOutRepos         []string `yaml:"optOutRepos"`         // List of repos to opt-out when in opt-out strategy
	DisableRepoOverride bool     `yaml:"disableRepoOverride"` // Set to true to disallow repos from opt-in/out in their config
}

type RepoConfig struct {
	OptConfig RepoOptConfig `yaml:"optConfig"`
}

type RepoOptConfig struct {
	OptIn  bool `yaml:"optIn"`  // Opt-in this repo when in opt-in strategy
	OptOut bool `yaml:"optOut"` // Opt-out this repo when in opt-out strategy
}

func GetOrgRepo() string {
	return config_OrgConfigRepo
}

func GetRepoDir() string {
	return config_RepoConfigDir
}

func FetchConfig(ctx context.Context, c *github.Client, owner, repo, path string, out interface{}) error {
	return fetchConfig(ctx, c.Repositories, owner, repo, path, out)
}

func fetchConfig(ctx context.Context, r repositories, owner, repo, path string, out interface{}) error {
	cf, _, _, err := r.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		return err
	}
	con, err := cf.GetContent()
	if err != nil {
		return err
	}
	if err := yaml.UnmarshalStrict([]byte(con), out); err != nil {
		log.Printf("Malformed config file %v/%v:%v\t%v", owner, repo, path, err)
		return err
	}
	return nil
}

type repositories interface {
	GetContents(context.Context, string, string, string,
		*github.RepositoryContentGetOptions) (*github.RepositoryContent,
		[]*github.RepositoryContent, *github.Response, error)
}

func IsEnabled(o OrgOptConfig, r RepoOptConfig, repo string) bool {
	var enabled bool
	if o.OptOutStrategy {
		enabled = true
		if contains(o.OptOutRepos, repo) {
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
	return enabled
}

func IsBotEnabled(ctx context.Context, c *github.Client, owner, repo string) bool {
	return isBotEnabled(ctx, c.Repositories, owner, repo)
}

func isBotEnabled(ctx context.Context, r repositories, owner, repo string) bool {
	// drop errors, if cfg file is not there, go with defaults
	oc := &OrgConfig{}
	fetchConfig(ctx, r, owner, config_OrgConfigRepo, config_ConfigFile, oc)
	rc := &RepoConfig{}
	fetchConfig(ctx, r, owner, repo, path.Join(config_RepoConfigDir, config_ConfigFile), rc)

	enabled := IsEnabled(oc.OptConfig, rc.OptConfig, repo)
	log.Printf("Repo enabled? %v / %v : %v", owner, repo, enabled)
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
