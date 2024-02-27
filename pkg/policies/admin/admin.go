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

// Package admin implements the Repository Administrators security policy.
package admin

import (
	"context"

	"github.com/gobwas/glob"
	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v59/github"
	"github.com/rs/zerolog/log"
)

const configFile = "admin.yaml"
const polName = "Repository Administrators"

const ownerlessText = `Did not find any owners of this repository
This policy requires all repositories to have a user or team assigned as an administrator.  A responsible party is required by organization policy to respond to security events and organization requests.

To add an administrator From the main page of the repository, go to Settings -> Manage Access.
(For more information, see https://docs.github.com/en/organizations/managing-access-to-your-organizations-repositories)

Alternately, if this repository does not have any maintainers, archive or delete it.
`

const userAdminsText = `Users are not allowed to be administrators of this repository.
Instead a team should be added as administrator. 

To add a team as administrator From the main page of the repository, go to Settings -> Manage Access.
(For more information, see https://docs.github.com/en/organizations/managing-access-to-your-organizations-repositories)
`

const maxNumberUserAdminsText = `The number of users with admin permission on this repository is greater than the allowed maximum value.
`

const teamAdminsText = `Teams are not allowed to be administrators of this repository.
Instead a user should be added as administrator. 

To add a user as administrator From the main page of the repository, go to Settings -> Manage Access.
(For more information, see https://docs.github.com/en/organizations/managing-access-to-your-organizations-repositories)
`

const maxNumberAdminTeamsText = `The number of teams with admin permission on this repository is greater than the allowed maximum value.
`

// OrgConfig is the org-level config definition for Repository Administrators
// security policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride
	// applies to all config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`

	// OwnerlessAllowed defines if repositories are allowed to have no
	// administrators, default false.
	OwnerlessAllowed bool `json:"ownerlessAllowed"`

	// Whether to allow users to be admins on a repo. If false then only teams can be admins. Default true.
	UserAdminsAllowed bool `json:"userAdminsAllowed"`

	// The maximum number of users with admin permissions on a repo that are allowed.
	// It only takes effect if a value > 0 is specified. If you wish to disallow user admins in general, please use the userAdminsAllowed bool instead.
	MaxNumberUserAdmins int `json:"maxNumberUserAdmins"`

	// Whether to allow teams to be admins on a repo. If false then only users can be admins. Default true.
	TeamAdminsAllowed bool `json:"teamAdminsAllowed"`

	// The maximum number of teams with admin permissions on a repo that are allowed.
	// It only takes effect if a value > 0 is specified. If you wish to disallow admin teams in general, please use the teamAdminsAllowed bool instead.
	MaxNumberAdminTeams int `json:"maxNumberAdminTeams"`

	// Exemptions is a list of repo-bool pairings to exempt.
	// Exemptions are only defined at the org level because they should be made
	// obvious to org security managers.
	Exemptions []*AdministratorExemption `json:"exemptions"`
}

// RepoConfig is the repo-level config for Repository Administrators security
// policy.
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `json:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `json:"action"`

	// OwnerlessAllowed overrides the same setting in org-level, only if present.
	OwnerlessAllowed *bool `json:"ownerlessAllowed"`

	// UserAdminsAllowed overrides the same setting in org-level, only if present.
	UserAdminsAllowed *bool `json:"userAdminsAllowed"`

	// MaxNumberUserAdmins overrides the same setting in org-level, only if present.
	MaxNumberUserAdmins *int `json:"maxNumberUserAdmins"`

	// TeamAdminsAllowed overrides the same setting in org-level, only if present.
	TeamAdminsAllowed *bool `json:"teamAdminsAllowed"`

	// MaxNumberAdminTeams overrides the same setting in org-level, only if present.
	MaxNumberAdminTeams *int `json:"maxNumberAdminTeams"`
}

type mergedConfig struct {
	Action              string
	OwnerlessAllowed    bool
	UserAdminsAllowed   bool
	TeamAdminsAllowed   bool
	MaxNumberAdminTeams int
	MaxNumberUserAdmins int
	Exemptions          []*AdministratorExemption
}

type globCache map[string]glob.Glob

// AdministratorExemption is an exemption entry for the Repository Administrators policy.
type AdministratorExemption struct {

	// Repo is a GitHub repo name. Globs are allowed.
	Repo string `json:"repo"`

	// OwnerlessAllowed defines if repositories are allowed to have no
	// administrators, default false.
	OwnerlessAllowed bool `json:"ownerlessAllowed"`

	// Whether to allow users to be admins on a repo. If false then only teams can be admins. Default true.
	UserAdminsAllowed bool `json:"userAdminsAllowed"`

	// Allow specific users to be admins on this repository. It overrides the boolean value UserAdminsAllowed.
	UserAdmins []string `json:"userAdmins"`

	// The maximum number of users with admin permissions on this repo that are allowed.  It overrides the int value MaxNumberUserAdmins.
	// It only takes effect if a value > 0 is specified. If you wish to disallow user admins in general, please use the userAdminsAllowed bool instead.
	MaxNumberUserAdmins int `json:"maxNumberUserAdmins"`

	// Whether to allow teams to be admins on a repo. If false then only users can be admins. Default true.
	TeamAdminsAllowed bool `json:"teamAdminsAllowed"`

	// Allow specific teams to be admins on this repository. It overrides the boolean value TeamAdminsAllowed.
	TeamAdmins []string `json:"teamAdmins"`

	// The maximum number of teams with admin permissions on this repo that are allowed. It overrides the int value MaxNumberAdminTeams.
	// It only takes effect if a value > 0 is specified. If you wish to disallow admin teams in general, please use the teamAdminsAllowed bool instead.
	MaxNumberAdminTeams int `json:"maxNumberAdminTeams"`
}

type details struct {
	Admins     []string
	TeamAdmins []string
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error

var configIsEnabled func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig, c *github.Client, owner, repo string) (bool, error)

func init() {
	configFetchConfig = config.FetchConfig
	configIsEnabled = config.IsEnabled
}

// Admin is the Repository Administrator policy object, implements policydef.Policy.
type Admin bool

// NewAdmin returns a new Repository Administrator policy.
func NewAdmin() policydef.Policy {
	var a Admin
	return a
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (a Admin) Name() string {
	return polName
}

type repositories interface {
	ListCollaborators(context.Context, string, string,
		*github.ListCollaboratorsOptions) ([]*github.User, *github.Response, error)
	ListTeams(context.Context, string, string, *github.ListOptions) (
		[]*github.Team, *github.Response, error)
}

// Check performs the policy check for Repository Administrators based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (a Admin) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	return check(ctx, c.Repositories, c, owner, repo)
}

// Check whether this policy is enabled or not
func (a Admin) IsEnabled(ctx context.Context, c *github.Client, owner, repo string) (bool, error) {
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

	mc := mergeConfig(oc, orc, rc, repo)

	gc := globCache{}

	var d details
	Admins, err := getAdminUsers(ctx, rep, owner, repo, mc.Exemptions, gc)
	if err != nil {
		return nil, err
	}
	d.Admins = Admins

	opt := &github.ListOptions{
		PerPage: 100,
	}
	var teams []*github.Team
	for {
		ts, resp, err := rep.ListTeams(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		teams = append(teams, ts...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	var teamAdmins []string
	for _, t := range teams {
		if t.GetPermissions()["admin"] {
			teamAdmins = append(teamAdmins, t.GetSlug())
		}
	}
	d.TeamAdmins = teamAdmins

	rv := &policydef.Result{
		Enabled: enabled,
		Pass:    true,
		Details: d,
	}

	// Test OwnerlessAllowed
	if (len(d.Admins)+len(d.TeamAdmins)) < 1 && !(mc.OwnerlessAllowed || isOwnerlessExempt(repo, mc.Exemptions, gc)) {
		rv.Pass = false
		rv.NotifyText = rv.NotifyText + ownerlessText
	}

	// Test UserAdminsAllowed
	if len(d.Admins) > 0 && !(mc.UserAdminsAllowed || isUserAdminsExempt(repo, d.Admins, mc.Exemptions, gc)) {
		rv.Pass = false
		rv.NotifyText = rv.NotifyText + userAdminsText
	}

	// Test MaxNumberUserAdmins exemption if it's defined
	if !isMaxNumberUserAdminsExempt(repo, len(d.Admins), mc.Exemptions, gc, true) {
		rv.Pass = false
		rv.NotifyText = rv.NotifyText + maxNumberUserAdminsText
	}

	// Test MaxNumberUserAdmins
	if mc.MaxNumberUserAdmins > 0 && len(d.Admins) > mc.MaxNumberUserAdmins && !isMaxNumberUserAdminsExempt(repo, len(d.Admins), mc.Exemptions, gc, false) {
		rv.Pass = false
		rv.NotifyText = rv.NotifyText + maxNumberUserAdminsText
	}

	// Test TeamAdminsAllowed
	if len(d.TeamAdmins) > 0 && !(mc.TeamAdminsAllowed || isTeamAdminsExempt(repo, d.TeamAdmins, mc.Exemptions, gc)) {
		rv.Pass = false
		rv.NotifyText = rv.NotifyText + teamAdminsText
	}

	// Test MaxNumberAdminTeams exemption if it's defined
	if !isMaxNumberAdminTeamsExempt(repo, len(d.TeamAdmins), mc.Exemptions, gc, true) {
		rv.Pass = false
		rv.NotifyText = rv.NotifyText + maxNumberAdminTeamsText
	}

	// Test MaxNumberAdminTeams
	if mc.MaxNumberAdminTeams > 0 && len(d.TeamAdmins) > mc.MaxNumberAdminTeams && !isMaxNumberAdminTeamsExempt(repo, len(d.TeamAdmins), mc.Exemptions, gc, false) {
		rv.Pass = false
		rv.NotifyText = rv.NotifyText + maxNumberAdminTeamsText
	}

	return rv, nil
}

func getAdminUsers(ctx context.Context, r repositories, owner, repo string,
	exemptions []*AdministratorExemption, gc globCache) ([]string, error) {
	opt := &github.ListCollaboratorsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
		Affiliation: "direct",
	}
	var users []*github.User
	for {
		us, resp, err := r.ListCollaborators(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		users = append(users, us...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	var rv []string
	for _, u := range users {
		if u.GetPermissions()["admin"] {
			rv = append(rv, u.GetLogin())
		}
	}
	return rv, nil
}

func isOwnerlessExempt(repo string, ee []*AdministratorExemption, gc globCache) bool {
	for _, e := range ee {
		g, err := gc.compileGlob(e.Repo)
		if err != nil {
			log.Warn().
				Str("repo", repo).
				Str("glob", e.Repo).
				Err(err).
				Msg("Unexpected error compiling the glob.")
		} else if g.Match(repo) && e.OwnerlessAllowed {
			return true
		}
	}
	return false
}

func isUserAdminsExempt(repo string, userAdmins []string, ee []*AdministratorExemption, gc globCache) bool {
	for _, e := range ee {
		g, err := gc.compileGlob(e.Repo)
		if err != nil {
			log.Warn().
				Str("repo", repo).
				Str("glob", e.Repo).
				Err(err).
				Msg("Unexpected error compiling the glob.")
		} else if g.Match(repo) && (e.UserAdminsAllowed || in(userAdmins, e.UserAdmins)) {
			return true
		}
	}
	return false
}

func isTeamAdminsExempt(repo string, teamAdmins []string, ee []*AdministratorExemption, gc globCache) bool {
	for _, e := range ee {
		g, err := gc.compileGlob(e.Repo)
		if err != nil {
			log.Warn().
				Str("repo", repo).
				Str("glob", e.Repo).
				Err(err).
				Msg("Unexpected error compiling the glob.")
		} else if g.Match(repo) && (e.TeamAdminsAllowed || in(teamAdmins, e.TeamAdmins)) {
			return true
		}
	}
	return false
}

func in(admins []string, list []string) bool {
	for _, admin := range admins {
		var found bool = false
		for _, l := range list {
			if l == admin {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func isMaxNumberUserAdminsExempt(repo string, adminsCount int, ee []*AdministratorExemption, gc globCache, def bool) bool {
	for _, e := range ee {
		g, err := gc.compileGlob(e.Repo)
		if err != nil {
			log.Warn().
				Str("repo", repo).
				Str("glob", e.Repo).
				Err(err).
				Msg("Unexpected error compiling the glob.")
		} else if g.Match(repo) && e.MaxNumberUserAdmins > 0 {
			return e.MaxNumberUserAdmins >= adminsCount
		}
	}
	return def
}

func isMaxNumberAdminTeamsExempt(repo string, teamAdminsCount int, ee []*AdministratorExemption, gc globCache, def bool) bool {
	for _, e := range ee {
		g, err := gc.compileGlob(e.Repo)
		if err != nil {
			log.Warn().
				Str("repo", repo).
				Str("glob", e.Repo).
				Err(err).
				Msg("Unexpected error compiling the glob.")
		} else if g.Match(repo) && e.MaxNumberAdminTeams > 0 {
			return e.MaxNumberAdminTeams >= teamAdminsCount
		}
	}
	return def
}

// Fix implementing policydef.Policy.Fix(). Currently not supported. Plan
// to support this TODO.
func (a Admin) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	log.Warn().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Msg("Action fix is configured, but not implemented.")
	return nil
}

// GetAction returns the configured action from this policy's
// configuration stored in the org-level repo, default log. Implementing
// policydef.Policy.GetAction()
func (a Admin) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, orc, rc, repo)
	return mc.Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig, *RepoConfig) {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action:              "log",
		OwnerlessAllowed:    false,
		UserAdminsAllowed:   true,
		MaxNumberUserAdmins: 0,
		TeamAdminsAllowed:   true,
		MaxNumberAdminTeams: 0,
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
		Action:              oc.Action,
		OwnerlessAllowed:    oc.OwnerlessAllowed,
		UserAdminsAllowed:   oc.UserAdminsAllowed,
		MaxNumberUserAdmins: oc.MaxNumberUserAdmins,
		TeamAdminsAllowed:   oc.TeamAdminsAllowed,
		MaxNumberAdminTeams: oc.MaxNumberAdminTeams,
		Exemptions:          oc.Exemptions,
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
	if rc.OwnerlessAllowed != nil {
		mc.OwnerlessAllowed = *rc.OwnerlessAllowed
	}
	if rc.UserAdminsAllowed != nil {
		mc.UserAdminsAllowed = *rc.UserAdminsAllowed
	}
	if rc.MaxNumberUserAdmins != nil {
		mc.MaxNumberUserAdmins = *rc.MaxNumberUserAdmins
	}
	if rc.TeamAdminsAllowed != nil {
		mc.TeamAdminsAllowed = *rc.TeamAdminsAllowed
	}
	if rc.MaxNumberAdminTeams != nil {
		mc.MaxNumberAdminTeams = *rc.MaxNumberAdminTeams
	}
	return mc
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
