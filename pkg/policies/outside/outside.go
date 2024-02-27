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

// Package Outside implements the Outside Collaborators security policy.
package outside

import (
	"context"
	"fmt"

	"github.com/gobwas/glob"
	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v59/github"
	"github.com/rs/zerolog/log"
)

const configFile = "outside.yaml"
const polName = "Outside Collaborators"

const accessText = "Found %v outside collaborators with %v access.\n"

const accessExp = `This policy requires users with this access to be members of the organisation. That way you can easily audit who has access to your repo, and if an account is compromised it can quickly be denied access to organization resources. To fix this you should either remove the user from repository-based access, or add them to the organization. 

* Remove the user from the repository-based access. From the main page of the repository, go to Settings -> Manage Access. 
(For more information, see https://docs.github.com/en/account-and-profile/setting-up-and-managing-your-github-user-account/managing-access-to-your-personal-repositories/removing-a-collaborator-from-a-personal-repository)

OR

* Invite the user to join your organisation. Click your profile photo and choose “Your Organization” → choose the org name → “People” → “Invite Member.” (For more information, see https://docs.github.com/en/organizations/managing-membership-in-your-organization/inviting-users-to-join-your-organization)

If you don't see the Settings tab you probably don't have administrative access. Reach out to the administrators of the organisation to fix this issue.

OR

* Exempt the user by adding an exemption to your organization-level Outside Collaborators configuration file.
`

// OrgConfig is the org-level config definition for Outside Collaborators
// security policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride
	// applies to all config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`

	// PushAllowed defined if outside collaborators are allowed to have push
	// access, default true.
	PushAllowed bool `json:"pushAllowed"`

	// AdminAllowed defined if outside collaborators are allowed to have admin
	// access, default false.
	AdminAllowed bool `json:"adminAllowed"`

	// Exemptions is a list of user-repo-access pairings to exempt.
	// Exemptions are only defined at the org level because they should be made
	// obvious to org security managers.
	Exemptions []*OutsideExemption `json:"exemptions"`
}

// RepoConfig is the repo-level config for Outside Collaborators security
// policy.
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `json:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `json:"action"`

	// PushAllowed overrides the same setting in org-level, only if present.
	PushAllowed *bool `json:"pushAllowed"`

	// AdminAllowed overrides the same setting in org-level, only if present.
	AdminAllowed *bool `json:"adminAllowed"`
}

type mergedConfig struct {
	Action       string
	PushAllowed  bool
	AdminAllowed bool
	Exemptions   []*OutsideExemption
}

type globCache map[string]glob.Glob

// OutsideExemption is an exemption entry for the Outside Collaborators policy.
type OutsideExemption struct {
	// User is a GitHub username
	User string `json:"user"`

	// Repo is a GitHub repo name
	Repo string `json:"repo"`

	// Push allows push permission
	Push bool `json:"push"`

	// Admin allows admin permission
	Admin bool `json:"admin"`
}

type details struct {
	OutsidePushCount  int
	OutsidePushers    []string
	OutsideAdminCount int
	OutsideAdmins     []string
	OwnerCount        int
	DirectOrgAdmins   []string
	TeamAdmins        []string
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error

var configIsEnabled func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig, c *github.Client, owner, repo string) (bool, error)

func init() {
	configFetchConfig = config.FetchConfig
	configIsEnabled = config.IsEnabled
}

// Outside is the Outside Collaborators policy object, implements policydef.Policy.
type Outside bool

// NewOutside returns a new Outside Collaborators policy.
func NewOutside() policydef.Policy {
	var o Outside
	return o
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (o Outside) Name() string {
	return polName
}

type repositories interface {
	ListCollaborators(context.Context, string, string,
		*github.ListCollaboratorsOptions) ([]*github.User, *github.Response, error)
	ListTeams(context.Context, string, string, *github.ListOptions) (
		[]*github.Team, *github.Response, error)
}

// Check performs the policy check for Outside Collaborators based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (o Outside) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	return check(ctx, c.Repositories, c, owner, repo)
}

// Check whether this policy is enabled or not
func (o Outside) IsEnabled(ctx context.Context, c *github.Client, owner, repo string) (bool, error) {
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
	outAdmins, err := getUsers(ctx, rep, owner, repo, "admin", "outside", mc.Exemptions, gc)
	if err != nil {
		return nil, err
	}
	outPushers, err := getUsers(ctx, rep, owner, repo, "push", "outside", mc.Exemptions, gc)
	if err != nil {
		return nil, err
	}
	d.OutsideAdminCount = len(outAdmins)
	d.OutsideAdmins = outAdmins
	d.OutsidePushCount = len(outPushers)
	d.OutsidePushers = outPushers

	directAdmins, err := getUsers(ctx, rep, owner, repo, "admin", "direct", mc.Exemptions, gc)
	if err != nil {
		return nil, err
	}
	var directOrgAdmins []string
	for _, a := range directAdmins {
		if !in(a, outAdmins) {
			directOrgAdmins = append(directOrgAdmins, a)
		}
	}
	d.OwnerCount = d.OwnerCount + len(directOrgAdmins)
	d.DirectOrgAdmins = directOrgAdmins

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
	d.OwnerCount = d.OwnerCount + len(teamAdmins)
	d.TeamAdmins = teamAdmins

	rv := &policydef.Result{
		Enabled: enabled,
		Pass:    true,
		Details: d,
	}

	exp := false
	if d.OutsidePushCount > 0 && !mc.PushAllowed {
		rv.Pass = false
		rv.NotifyText = rv.NotifyText +
			fmt.Sprintf(accessText, d.OutsidePushCount, "push")
		exp = true
	}
	if d.OutsideAdminCount > 0 && !mc.AdminAllowed {
		rv.Pass = false
		rv.NotifyText = rv.NotifyText +
			fmt.Sprintf(accessText, d.OutsideAdminCount, "admin")
		exp = true
	}
	if exp {
		rv.NotifyText = rv.NotifyText + accessExp
	}
	return rv, nil
}

func in(name string, list []string) bool {
	for _, v := range list {
		if v == name {
			return true
		}
	}
	return false
}

func getUsers(ctx context.Context, r repositories, owner, repo, perm,
	aff string, exemptions []*OutsideExemption, gc globCache) ([]string, error) {
	opt := &github.ListCollaboratorsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
		Affiliation: aff,
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
		if u.GetPermissions()[perm] {
			if !isExempt(repo, u.GetLogin(), perm, exemptions, gc) {
				rv = append(rv, u.GetLogin())
			}
		}
	}
	return rv, nil
}

func isExempt(repo, user, access string, ee []*OutsideExemption, gc globCache) bool {
	for _, e := range ee {
		if !(((e.Push || e.Admin) && access == "push") || (e.Admin && access == "admin")) {
			continue
		}
		g, err := gc.compileGlob(e.Repo)
		if err != nil {
			log.Warn().
				Str("repo", repo).
				Str("glob", e.Repo).
				Err(err).
				Msg("Unexpected error compiling the glob.")
		} else if g.Match(repo) && e.User == user {
			return true
		}
	}
	return false
}

// Fix implementing policydef.Policy.Fix(). Currently not supported. Plan
// to support this TODO.
func (o Outside) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
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
func (o Outside) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc, orc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, orc, rc, repo)
	return mc.Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig, *RepoConfig) {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action:      "log",
		PushAllowed: true,
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
		Action:       oc.Action,
		PushAllowed:  oc.PushAllowed,
		AdminAllowed: oc.AdminAllowed,
		Exemptions:   oc.Exemptions,
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
	if rc.PushAllowed != nil {
		mc.PushAllowed = *rc.PushAllowed
	}
	if rc.AdminAllowed != nil {
		mc.AdminAllowed = *rc.AdminAllowed
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
