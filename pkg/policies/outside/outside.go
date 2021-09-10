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
	"path"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v32/github"
	"github.com/rs/zerolog/log"
)

const configFile = "outside.yaml"
const polName = "Outside Collaborators"

// OrgConfig is the org-level config definition for Outside Collaborators
// security policy.
type OrgConfig struct {
	// OptConfig is the standard org-level opt in/out config, RepoOverride
	// applies to all config.
	OptConfig config.OrgOptConfig `yaml:"optConfig"`

	// Action defines which action to take, default log, other: issue...
	Action string `yaml:"action"`

	// PushAllowed defined if outside collaboraters are allowed to have push
	// access, default true.
	PushAllowed bool `yaml:"pushAllowed"`

	// AdminAllowed defined if outside collaboraters are allowed to have admin
	// access, default false.
	AdminAllowed bool `yaml:"adminAllowed"`
}

// RepoConfig is the repo-level config for Outside Collaborators security
// policy.
type RepoConfig struct {
	// OptConfig is the standard repo-level opt in/out config.
	OptConfig config.RepoOptConfig `yaml:"optConfig"`

	// Action overrides the same setting in org-level, only if present.
	Action *string `yaml:"action"`

	// PushAllowed overrides the same setting in org-level, only if present.
	PushAllowed *bool `yaml:"pushAllowed"`

	// AdminAllowed overrides the same setting in org-level, only if present.
	AdminAllowed *bool `yaml:"adminAllowed"`
}

type mergedConfig struct {
	Action       string
	PushAllowed  bool
	AdminAllowed bool
}

type details struct {
	OutsidePushCount  int
	OutsidePushers    []string
	OutsideAdminCount int
	OutsideAdmins     []string
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, interface{}) error

func init() {
	configFetchConfig = config.FetchConfig
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
}

// Check performs the polcy check for Outside Collaborators based on the
// configuration stored in the org/repo, implementing policydef.Policy.Check()
func (o Outside) Check(ctx context.Context, c *github.Client, owner,
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
	mc := mergeConfig(oc, rc, repo)

	opt := &github.ListCollaboratorsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
		Affiliation: "outside",
	}
	var users []*github.User
	for {
		us, resp, err := rep.ListCollaborators(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		users = append(users, us...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	var d details
	for _, u := range users {
		if u.GetPermissions()["push"] {
			d.OutsidePushCount = d.OutsidePushCount + 1
			d.OutsidePushers = append(d.OutsidePushers, u.GetLogin())
		}
		if u.GetPermissions()["admin"] {
			d.OutsideAdminCount = d.OutsideAdminCount + 1
			d.OutsideAdmins = append(d.OutsideAdmins, u.GetLogin())
		}
	}

	if d.OutsidePushCount > 0 && !mc.PushAllowed {
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       false,
			NotifyText: fmt.Sprintf(`Found %v outside collaborators with push access.
This policy requires all users with push access to be members of the organisation. That way you can easily audit who has access to your repo, and if an account is compromised it can quickly be denied access to organization resources. To fix this you should either remove the user from repository-based access, or add them to the organization. 

* Remove the user from the repository-based access. From the main page of the repository, go to Settings -> Manage Access. 
(For more information, see https://docs.github.com/en/account-and-profile/setting-up-and-managing-your-github-user-account/managing-access-to-your-personal-repositories/removing-a-collaborator-from-a-personal-repository)

OR

* Invite the user to join your organisation. Click your profile photo and choose “Your Organization” → choose the org name → “People” → “Invite Member.” (For more information, see https://docs.github.com/en/organizations/managing-membership-in-your-organization/inviting-users-to-join-your-organization)

If you don't see the Settings tab you probably don't have administrative access. Reach out to the administrators of the organisation to fix this issue.`, d.OutsidePushCount),
			Details:    d,
		}, nil
	}
	if d.OutsideAdminCount > 0 && !mc.AdminAllowed {
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       false,
			NotifyText: fmt.Sprintf(`Found %v outside collaborators with admin access.
This policy requires all users with admin access to be members of the organisation. That way you can easily audit who has access to your repo, and if an account is compromised it can quickly be denied access to organization resources. To fix this you should either remove the user from repository-based access, or add them to the organization. 

* Remove the user from the repository-based access. From the main page of the repository, go to Settings -> Manage Access. 
(For more information, see https://docs.github.com/en/account-and-profile/setting-up-and-managing-your-github-user-account/managing-access-to-your-personal-repositories/removing-a-collaborator-from-a-personal-repository)

OR

* Invite the user to join your organisation. Click your profile photo and choose “Your Organization” → choose the org name → “People” → “Invite Member.” (For more information, see https://docs.github.com/en/organizations/managing-membership-in-your-organization/inviting-users-to-join-your-organization)

If you don't see the Settings tab you probably don't have administrative access. Reach out to the administrators of the organisation to fix this issue.`, d.OutsideAdminCount),
			Details:    d,
		}, nil
	}
	return &policydef.Result{
		Enabled:    enabled,
		Pass:       true,
		NotifyText: "",
		Details:    d,
	}, nil
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
	oc, rc := getConfig(ctx, c, owner, repo)
	mc := mergeConfig(oc, rc, repo)
	return mc.Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) (*OrgConfig, *RepoConfig) {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action:      "log",
		PushAllowed: true,
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
		Action:       oc.Action,
		PushAllowed:  oc.PushAllowed,
		AdminAllowed: oc.AdminAllowed,
	}

	if !oc.OptConfig.DisableRepoOverride {
		if rc.Action != nil {
			mc.Action = *rc.Action
		}
		if rc.PushAllowed != nil {
			mc.PushAllowed = *rc.PushAllowed
		}
		if rc.AdminAllowed != nil {
			mc.AdminAllowed = *rc.AdminAllowed
		}
	}
	return mc
}
