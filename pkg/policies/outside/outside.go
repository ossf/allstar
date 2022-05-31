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

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v43/github"
	"github.com/rs/zerolog/log"
)

var doNothingOnOptOut = operator.DoNothingOnOptOut

const configFile = "outside.yaml"
const polName = "Outside Collaborators"

const accessText = "Found %v outside collaborators with %v access.\n"

const accessExp = `This policy requires all users with this access to be members of the organisation. That way you can easily audit who has access to your repo, and if an account is compromised it can quickly be denied access to organization resources. To fix this you should either remove the user from repository-based access, or add them to the organization. 

* Remove the user from the repository-based access. From the main page of the repository, go to Settings -> Manage Access. 
(For more information, see https://docs.github.com/en/account-and-profile/setting-up-and-managing-your-github-user-account/managing-access-to-your-personal-repositories/removing-a-collaborator-from-a-personal-repository)

OR

* Invite the user to join your organisation. Click your profile photo and choose “Your Organization” → choose the org name → “People” → “Invite Member.” (For more information, see https://docs.github.com/en/organizations/managing-membership-in-your-organization/inviting-users-to-join-your-organization)

If you don't see the Settings tab you probably don't have administrative access. Reach out to the administrators of the organisation to fix this issue.
`

const ownerlessText = `Did not find any owners of this repository
This policy requires all repositories to have an organization member or team assigned as an administrator. Either there are no administrators, or all administrators are outside collaborators. A responsible party is required by organization policy to respond to security events and organization requests.

To add an administrator From the main page of the repository, go to Settings -> Manage Access.
(For more information, see https://docs.github.com/en/organizations/managing-access-to-your-organizations-repositories)

Alternately, if this repository does not have any maintainers, archive or delete it.
`

// OrgConfig is the org-level config definition for Outside Collaborators
// security policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride
	// applies to all config.
	OptConfig config.OrgOptConfig `json:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`

	// PushAllowed defined if outside collaboraters are allowed to have push
	// access, default true.
	PushAllowed bool `json:"pushAllowed"`

	// AdminAllowed defined if outside collaboraters are allowed to have admin
	// access, default false.
	AdminAllowed bool `json:"adminAllowed"`

	// TestingOwnerlessAllowed defined if repositories are allowed to have no
	// administrators, default false.
	TestingOwnerlessAllowed bool `json:"testingOwnerlessAllowed"`
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

	// TestingOwnerlessAllowed overrides the same setting in org-level, only if present.
	TestingOwnerlessAllowed *bool `json:"testingOwnerlessAllowed"`
}

type mergedConfig struct {
	Action                  string
	PushAllowed             bool
	AdminAllowed            bool
	TestingOwnerlessAllowed bool
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

// Check performs the polcy check for Outside Collaborators based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (o Outside) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	return check(ctx, c.Repositories, c, owner, repo)
}

func check(ctx context.Context, rep repositories, c *github.Client, owner,
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
	if !enabled && doNothingOnOptOut {
		// Don't run this policy if disabled and requested by operator. This is
		// only checking enablement of policy, but not Allstar overall, this is
		// ok for now.
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       true,
			NotifyText: "Disabled",
			Details:    details{},
		}, nil
	}

	mc := mergeConfig(oc, orc, rc, repo)

	var d details
	outAdmins, err := getUsers(ctx, rep, owner, repo, "admin", "outside")
	if err != nil {
		return nil, err
	}
	outPushers, err := getUsers(ctx, rep, owner, repo, "push", "outside")
	if err != nil {
		return nil, err
	}
	d.OutsideAdminCount = len(outAdmins)
	d.OutsideAdmins = outAdmins
	d.OutsidePushCount = len(outPushers)
	d.OutsidePushers = outPushers

	directAdmins, err := getUsers(ctx, rep, owner, repo, "admin", "direct")
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

	// FIXME Ownerless not working due to bug in List Teams GitHub API
	if d.OwnerCount == 0 && !mc.TestingOwnerlessAllowed {
		rv.Pass = false
		rv.NotifyText = rv.NotifyText + ownerlessText
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
	aff string) ([]string, error) {
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
			rv = append(rv, u.GetLogin())
		}
	}
	return rv, nil
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
		Action:                  "log",
		PushAllowed:             true,
		TestingOwnerlessAllowed: true,
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
		Action:                  oc.Action,
		PushAllowed:             oc.PushAllowed,
		AdminAllowed:            oc.AdminAllowed,
		TestingOwnerlessAllowed: oc.TestingOwnerlessAllowed,
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
	if rc.TestingOwnerlessAllowed != nil {
		mc.TestingOwnerlessAllowed = *rc.TestingOwnerlessAllowed
	}
	return mc
}
