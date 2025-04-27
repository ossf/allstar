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
	"encoding/json"
	"net/http"
	"path"
	"strings"

	"github.com/gobwas/glob"
	"github.com/ossf/allstar/pkg/config/operator"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/google/go-github/v59/github"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

// OrgConfig is the org-level config definition for Allstar
type OrgConfig struct {
	// OptConfig contains the opt in/out configuration.
	OptConfig OrgOptConfig `json:"optConfig"`

	// IssueLabel is the label used to tag, search, and identify GitHub Issues
	// created by the bot. The default is specified by the operator of Allstar,
	// currently: "allstar"
	IssueLabel string `json:"issueLabel"`

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
	IssueRepo string `json:"issueRepo"`

	// IssueFooter is a custom message to add to the end of all Allstar created
	// issues in the GitHub organization. It does not supercede the bot-level
	// footer (found in pkg/config/operator) but is added in addition to that
	// one. This setting is useful to direct users to the organization-level
	// config repository or documentation describing your Allstar settings and
	// policies.
	IssueFooter string `json:"issueFooter"`

	// Schedule specifies whether to perform certain actions on specific days.
	Schedule *ScheduleConfig `json:"schedule"`
}

// OrgOptConfig is used in Allstar and policy-specific org-level config to
// define the opt in/out configuration.
type OrgOptConfig struct {
	// OptOutStrategy : set to true to change from opt-in to opt-out.
	OptOutStrategy bool `json:"optOutStrategy"`

	// OptInRepos is the list of repos to opt-in when in opt-in strategy.
	OptInRepos []string `json:"optInRepos"`

	// OptOutRepos is the list of repos to opt-out when in opt-out strategy.
	OptOutRepos []string `json:"optOutRepos"`

	// OptOutPrivateRepos : set to true to not access private repos.
	OptOutPrivateRepos bool `json:"optOutPrivateRepos"`

	// OptOutPublicRepos : set to true to not access public repos.
	OptOutPublicRepos bool `json:"optOutPublicRepos"`

	// OptOutArchivedRepos : set to true to opt-out archived repositories.
	OptOutArchivedRepos bool `json:"optOutArchivedRepos"`

	// OptOutForkedRepos : set to true to opt-out forked repositories.
	OptOutForkedRepos bool `json:"optOutForkedRepos"`

	// DisableRepoOverride : set to true to disallow repos from opt-in/out in
	// their config.
	DisableRepoOverride bool `json:"disableRepoOverride"`
}

// RepoConfig is the repo-level config definition for Allstar
type RepoConfig struct {
	// OptConfig contains the opt in/out configuration.
	OptConfig RepoOptConfig `json:"optConfig"`

	// IssueLabel is the label used to tag, search, and identify GitHub Issues
	// created by the bot. Repo-level label my override Org-level setting
	// regardless of Optconfig.DisableRepoOverride.
	IssueLabel string `json:"issueLabel"`

	// Schedule specifies days during which to not send notifications,
	Schedule *ScheduleConfig `json:"schedule"`
}

// RepoOptConfig is used in Allstar and policy-specific repo-level config to
// opt in/out of enforcement.
type RepoOptConfig struct {
	// OptIn : set to true to opt-in this repo when in opt-in strategy
	OptIn bool `json:"optIn"`

	// OptOut: set to true to opt-out this repo when in opt-out strategy
	OptOut bool `json:"optOut"`
}

// ScheduleConfig is used to disable notifications during specific days,
// such as weekends.
type ScheduleConfig struct {
	// Timezone specifies a timezone, eg. "America/Los_Angeles"
	// See https://en.wikipedia.org/wiki/List_of_tz_database_time_zones#List
	Timezone string `json:"timezone"`

	// Days specifies up to three weekdays during which to disable pings.
	// eg. "saturday" or "sunday"
	Days []string `json:"days"`
}

type globCache map[string]glob.Glob

const githubConfRepo = ".github"

// ConfigLevel is an enum to indicate which level config to retrieve for the
// particular policy.
type ConfigLevel int8

const (
	// OrgLevel is the organization level config that is defined in the .allstar
	// or .github config repo.
	OrgLevel ConfigLevel = iota

	// OrgRepoLevel is the repo level config that is defined in the .allstar or
	// .github config repo.
	OrgRepoLevel

	// RepoLevel is the repo level config that is defined in the .allstar folder
	// of the repo being checked.
	RepoLevel
)

var walkGC func(context.Context, repositories, string, string, string,
	*github.RepositoryContentGetOptions) (*github.RepositoryContent,
	[]*github.RepositoryContent, *github.Response, error)

var gc = globCache{}

func init() {
	walkGC = walkGetContents
}

type repositories interface {
	Get(context.Context, string, string) (*github.Repository,
		*github.Response, error)
	GetContents(context.Context, string, string, string,
		*github.RepositoryContentGetOptions) (*github.RepositoryContent,
		[]*github.RepositoryContent, *github.Response, error)
}

// FetchConfig grabs a yaml config file from github and writes it to out.
func FetchConfig(ctx context.Context, c *github.Client, owner, repo, name string, cl ConfigLevel, out interface{}) error {
	return fetchConfig(ctx, c.Repositories, owner, repo, name, cl, out)
}

func fetchConfig(ctx context.Context, r repositories, owner, repoIn, name string, cl ConfigLevel, out interface{}) error {
	il, err := getInstLoc(ctx, r, owner)
	if err != nil {
		return err
	}
	var repo string
	var p string
	switch cl {
	case OrgLevel:
		if !il.Exists {
			return nil
		}
		repo = il.Repo
		p = path.Join(il.Path, name)
	case OrgRepoLevel:
		if !il.Exists {
			return nil
		}
		repo = il.Repo
		p = path.Join(il.Path, repoIn, name)
	case RepoLevel:
		repo = repoIn
		p = path.Join(operator.RepoConfigDir, name)
	}
	cf, _, rsp, err := walkGC(ctx, r, owner, repo, p, nil)
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
	conJSON, err := yaml.YAMLToJSON([]byte(con))
	if err != nil {
		return err
	}
	if cl == OrgLevel {
		mergedJSON, err := checkAndMergeBase(ctx, r, p, conJSON)
		if err != nil {
			return err
		}
		conJSON = mergedJSON
	}
	if err := json.Unmarshal(conJSON, out); err != nil {
		log.Warn().
			Str("org", owner).
			Str("repo", repo).
			Str("file", p).
			Err(err).
			Msg("Malformed config file, using defaults.")
		return nil
	}
	return nil
}

type anyWithBase struct {
	BaseConfig *string `json:"baseConfig"`
}

// checkAndMergeBase checks the contents for a field "baseConfig". If found
// reads that as "org/repo" then pulls the same path from there and uses it as
// a base config to merge this contents on top of. Returns JSON.
func checkAndMergeBase(ctx context.Context, r repositories, path string, contents []byte) ([]byte, error) {
	var b anyWithBase
	if err := json.Unmarshal(contents, &b); err != nil {
		return nil, err
	}
	if b.BaseConfig == nil {
		return contents, nil
	}
	sp := strings.Split(*b.BaseConfig, "/")
	if len(sp) != 2 {
		log.Warn().
			Str("file", path).
			Str("baseConfig", *b.BaseConfig).
			Msg("Expect baseConfig to be a GitHub \"owner/repo\", ignoring.")
		return contents, nil
	}
	cf, _, rsp, err := r.GetContents(ctx, sp[0], sp[1], path, nil)
	if err != nil {
		if rsp != nil && rsp.StatusCode == http.StatusNotFound {
			log.Warn().
				Str("file", path).
				Str("baseConfig", *b.BaseConfig).
				Msg("Path in specified baseConfig does not exist.")
			return contents, nil
		}
		return nil, err
	}
	baseYAML, err := cf.GetContent()
	if err != nil {
		return nil, err
	}
	baseJSON, err := yaml.YAMLToJSON([]byte(baseYAML))
	if err != nil {
		return nil, err
	}
	if string(baseJSON) == "null" {
		baseJSON = []byte("{}")
	}
	mergedJSON, err := jsonpatch.MergePatch(baseJSON, contents)
	if err != nil {
		return nil, err
	}
	return mergedJSON, nil
}

// IsEnabled determines if a repo is enabled by interpreting the provided
// org-level, org-repo-level, and repo-level OptConfigs.
func IsEnabled(ctx context.Context, o OrgOptConfig, orc, r RepoOptConfig, c *github.Client, owner, repo string) (bool, error) {
	return isEnabled(ctx, o, orc, r, c.Repositories, owner, repo)
}

func isEnabled(ctx context.Context, o OrgOptConfig, orc, r RepoOptConfig, rep repositories, owner, repo string) (bool, error) {
	var enabled bool

	gr, _, err := rep.Get(ctx, owner, repo)
	if err != nil {
		return false, err
	}

	if o.OptOutStrategy {
		enabled = true
		if matches(o.OptOutRepos, repo, gc) {
			enabled = false
		}
		if o.OptOutPrivateRepos && gr.GetPrivate() {
			enabled = false
		}
		if o.OptOutPublicRepos && !gr.GetPrivate() {
			enabled = false
		}
		if o.OptOutArchivedRepos && gr.GetArchived() {
			enabled = false
		}
		if o.OptOutForkedRepos && gr.GetFork() {
			enabled = false
		}
		if orc.OptOut {
			enabled = false
		}
		if !o.DisableRepoOverride && r.OptOut {
			enabled = false
		}
	} else {
		enabled = false
		if matches(o.OptInRepos, repo, gc) {
			enabled = true
		}
		if orc.OptIn {
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
	oc, orc, rc := getAppConfigs(ctx, r, owner, repo)
	enabled, err := isEnabled(ctx, oc.OptConfig, orc.OptConfig, rc.OptConfig, r, owner, repo)
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
func GetAppConfigs(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig, *RepoConfig) {
	return getAppConfigs(ctx, c.Repositories, owner, repo)
}

func getAppConfigs(ctx context.Context, r repositories, owner, repo string) (*OrgConfig, *RepoConfig, *RepoConfig) {
	// drop errors, if cfg file is not there, go with defaults
	oc := &OrgConfig{}
	if err := fetchConfig(ctx, r, owner, "", operator.AppConfigFile, OrgLevel, oc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Str("configLevel", "orgLevel").
			Str("area", "bot").
			Str("file", operator.AppConfigFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	orc := &RepoConfig{}
	if err := fetchConfig(ctx, r, owner, repo, operator.AppConfigFile, OrgRepoLevel, orc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Str("configLevel", "orgRepoLevel").
			Str("area", "bot").
			Str("file", operator.AppConfigFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	rc := &RepoConfig{}
	if err := fetchConfig(ctx, r, owner, repo, operator.AppConfigFile, RepoLevel, rc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Str("configLevel", "repoLevel").
			Str("area", "bot").
			Str("file", operator.AppConfigFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	return oc, orc, rc
}

func matches(s []string, e string, gc globCache) bool {
	for _, v := range s {
		g, err := gc.compileGlob(v)
		if err != nil {
			log.Warn().
				Str("repo", e).
				Str("glob", v).
				Err(err).
				Msg("Unexpected error compiling the glob.")
		} else if g.Match(e) {
			return true
		}

	}
	return false
}

// compileGlob returns cached glob if present, otherwise attempts glob.Compile.
func (g globCache) compileGlob(s string) (glob.Glob, error) {
	if glob, ok := g[s]; ok {
		return glob, nil
	}
	c, err := glob.Compile(s)
	if err != nil {
		return nil, err
	}
	g[s] = c
	return c, nil
}
