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

package admin

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v59/github"
	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
)

var listCollaborators func(context.Context, string, string,
	*github.ListCollaboratorsOptions) ([]*github.User, *github.Response, error)
var listTeams func(context.Context, string, string, *github.ListOptions) (
	[]*github.Team, *github.Response, error)

type mockRepos struct{}

func (m mockRepos) ListCollaborators(ctx context.Context, o, r string,
	op *github.ListCollaboratorsOptions) ([]*github.User, *github.Response, error) {
	return listCollaborators(ctx, o, r, op)
}

func (m mockRepos) ListTeams(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.Team, *github.Response, error) {
	return listTeams(ctx, owner, repo, opts)
}

func TestConfigPrecedence(t *testing.T) {
	tests := []struct {
		Name      string
		Org       OrgConfig
		OrgRepo   RepoConfig
		Repo      RepoConfig
		ExpAction string
		Exp       mergedConfig
	}{
		{
			Name: "OrgOnly",
			Org: OrgConfig{
				Action:              "issue",
				OwnerlessAllowed:    true,
				UserAdminsAllowed:   true,
				MaxNumberUserAdmins: 1,
				TeamAdminsAllowed:   true,
				MaxNumberAdminTeams: 2,
			},
			OrgRepo:   RepoConfig{},
			Repo:      RepoConfig{},
			ExpAction: "issue",
			Exp: mergedConfig{
				Action:              "issue",
				OwnerlessAllowed:    true,
				UserAdminsAllowed:   true,
				MaxNumberUserAdmins: 1,
				TeamAdminsAllowed:   true,
				MaxNumberAdminTeams: 2,
			},
		},
		{
			Name: "OrgRepoOverOrg",
			Org: OrgConfig{
				Action:              "issue",
				OwnerlessAllowed:    true,
				UserAdminsAllowed:   true,
				MaxNumberUserAdmins: 1,
				TeamAdminsAllowed:   true,
				MaxNumberAdminTeams: 2,
			},
			OrgRepo: RepoConfig{
				Action:              github.String("log"),
				OwnerlessAllowed:    github.Bool(false),
				UserAdminsAllowed:   github.Bool(false),
				MaxNumberUserAdmins: github.Int(4),
				TeamAdminsAllowed:   github.Bool(false),
				MaxNumberAdminTeams: github.Int(3),
			},
			Repo:      RepoConfig{},
			ExpAction: "log",
			Exp: mergedConfig{
				Action:              "log",
				OwnerlessAllowed:    false,
				UserAdminsAllowed:   false,
				MaxNumberUserAdmins: 4,
				TeamAdminsAllowed:   false,
				MaxNumberAdminTeams: 3,
			},
		},
		{
			Name: "RepoOverAllOrg",
			Org: OrgConfig{
				Action: "issue",
			},
			OrgRepo: RepoConfig{
				Action:              github.String("log"),
				OwnerlessAllowed:    github.Bool(true),
				UserAdminsAllowed:   github.Bool(true),
				MaxNumberUserAdmins: github.Int(1),
				TeamAdminsAllowed:   github.Bool(true),
				MaxNumberAdminTeams: github.Int(2),
			},
			Repo: RepoConfig{
				Action:              github.String("email"),
				OwnerlessAllowed:    github.Bool(false),
				UserAdminsAllowed:   github.Bool(false),
				MaxNumberUserAdmins: github.Int(4),
				TeamAdminsAllowed:   github.Bool(false),
				MaxNumberAdminTeams: github.Int(3),
			},
			ExpAction: "email",
			Exp: mergedConfig{
				Action:              "email",
				MaxNumberUserAdmins: 4,
				MaxNumberAdminTeams: 3,
			},
		},
		{
			Name: "RepoDisallowed",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					DisableRepoOverride: true,
				},
				Action: "issue",
			},
			OrgRepo: RepoConfig{
				Action:              github.String("log"),
				OwnerlessAllowed:    github.Bool(true),
				UserAdminsAllowed:   github.Bool(true),
				MaxNumberUserAdmins: github.Int(1),
				TeamAdminsAllowed:   github.Bool(true),
				MaxNumberAdminTeams: github.Int(2),
			},
			Repo: RepoConfig{
				Action:              github.String("email"),
				OwnerlessAllowed:    github.Bool(false),
				UserAdminsAllowed:   github.Bool(false),
				MaxNumberUserAdmins: github.Int(4),
				TeamAdminsAllowed:   github.Bool(false),
				MaxNumberAdminTeams: github.Int(3),
			},
			ExpAction: "log",
			Exp: mergedConfig{
				Action:              "log",
				OwnerlessAllowed:    true,
				UserAdminsAllowed:   true,
				MaxNumberUserAdmins: 1,
				TeamAdminsAllowed:   true,
				MaxNumberAdminTeams: 2,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configFetchConfig = func(ctx context.Context, c *github.Client,
				owner, repo, path string, ol config.ConfigLevel, out interface{}) error {
				switch ol {
				case config.RepoLevel:
					rc := out.(*RepoConfig)
					*rc = test.Repo
				case config.OrgRepoLevel:
					orc := out.(*RepoConfig)
					*orc = test.OrgRepo
				case config.OrgLevel:
					oc := out.(*OrgConfig)
					*oc = test.Org
				}
				return nil
			}

			o := Admin(true)
			ctx := context.Background()

			action := o.GetAction(ctx, nil, "", "thisrepo")
			if action != test.ExpAction {
				t.Errorf("Unexpected results. want %s, got %s", test.ExpAction, action)
			}

			oc, orc, rc := getConfig(ctx, nil, "", "thisrepo")
			mc := mergeConfig(oc, orc, rc, "thisrepo")
			if diff := cmp.Diff(&test.Exp, mc); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	bob := "bob"
	alice := "alice"
	dave := "dave"
	tests := []struct {
		Name         string
		Org          OrgConfig
		Repo         RepoConfig
		Users        []*github.User
		cofigEnabled bool
		Exp          policydef.Result
		Teams        []*github.Team
	}{
		{
			Name: "NotEnabled",
			Org: OrgConfig{
				OwnerlessAllowed:  true,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo:         RepoConfig{},
			Users:        nil,
			cofigEnabled: false,
			Exp: policydef.Result{
				Enabled:    false,
				Pass:       true,
				NotifyText: "",
				Details:    details{},
			},
		},
		{
			Name: "Ownerless not allowed and fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  false,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Did not find any owners of this repository\nThis policy requires all repositories to have an organization member or team assigned as an administrator",
				Details: details{
					Admins: nil,
				},
			},
		},
		{
			Name: "Ownerless not allowed and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  false,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: []string{"bob"},
				},
			},
		},
		{
			Name: "Ownerless allowed and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: nil,
				},
			},
		},
		{
			Name: "Ownerless allowed and pass 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: []string{"bob"},
				},
			},
		},
		{
			Name: "Ownerless not allowed and fail (team)",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  false,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Did not find any owners of this repository\nThis policy requires all repositories to have an organization member or team assigned as an administrator",
				Details: details{
					TeamAdmins: nil,
				},
			},
		},
		{
			Name: "Ownerless not allowed and pass (team)",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  false,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					TeamAdmins: []string{"bob"},
				},
			},
		},
		{
			Name: "Ownerless allowed and pass (team)",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					TeamAdmins: nil,
				},
			},
		},
		{
			Name: "Ownerless allowed and pass (team) 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					TeamAdmins: []string{"bob"},
				},
			},
		},
		{
			Name: "Ownerless not allowed and pass (individual)",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  false,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins:     []string{"bob"},
					TeamAdmins: nil,
				},
			},
		},
		{
			Name: "Ownerless not allowed and pass (team)",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  false,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins:     nil,
					TeamAdmins: []string{"bob"},
				},
			},
		},
		{
			Name: "Ownerless not allowed but allowed by an exemption and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  false,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
				Exemptions: []*AdministratorExemption{
					{
						Repo:             "thisrepo",
						OwnerlessAllowed: true,
					},
				},
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: nil,
				},
			},
		},
		{
			Name: "Ownerless not allowed by an exemption and fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  false,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
				Exemptions: []*AdministratorExemption{
					{
						Repo:             "thisrepo",
						OwnerlessAllowed: false,
					},
				},
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Did not find any owners of this repository\nThis policy requires all repositories to have an organization member or team assigned as an administrator",
				Details: details{
					Admins: nil,
				},
			},
		},
		{
			Name: "UserAdminsAllowed not allowed and fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Users are not allowed to be administrators of this repository.\nInstead a team should be added as administrator.",
				Details: details{
					Admins: []string{"bob"},
				},
			},
		},
		{
			Name: "UserAdminsAllowed not allowed and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: nil,
				},
			},
		},
		{
			Name: "UserAdminsAllowed not allowed and pass 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins:     nil,
					TeamAdmins: []string{"bob"},
				},
			},
		},
		{
			Name: "UserAdminsAllowed allowed and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: []string{"bob"},
				},
			},
		},
		{
			Name: "UserAdminsAllowed allowed and pass 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: nil,
				},
			},
		},
		{
			Name: "UserAdminsAllowed not allowed but allowed by an exemption and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: true,
				Exemptions: []*AdministratorExemption{
					{
						Repo:              "thisrepo",
						OwnerlessAllowed:  true,
						UserAdminsAllowed: true,
					},
				},
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: []string{"bob"},
				},
			},
		},
		{
			Name: "UserAdminsAllowed not allowed but allowed by an exemption and pass 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: true,
				Exemptions: []*AdministratorExemption{
					{
						Repo:             "thisrepo",
						OwnerlessAllowed: true,
						UserAdmins:       []string{"dave", "bob"},
					},
				},
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: []string{"bob"},
				},
			},
		},
		{
			Name: "UserAdminsAllowed not allowed by by an exemption and fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: true,
				Exemptions: []*AdministratorExemption{
					{
						Repo:              "thisrepo",
						OwnerlessAllowed:  true,
						UserAdminsAllowed: false,
						UserAdmins:        []string{"dave"},
					},
				},
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Users are not allowed to be administrators of this repository.\nInstead a team should be added as administrator.",
				Details: details{
					Admins: []string{"bob"},
				},
			},
		},
		{
			Name: "UserAdminsAllowed not allowed by by an exemption and fail 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: true,
				Exemptions: []*AdministratorExemption{
					{
						Repo:              "thisrepo",
						OwnerlessAllowed:  true,
						UserAdminsAllowed: false,
					},
				},
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Users are not allowed to be administrators of this repository.\nInstead a team should be added as administrator.",
				Details: details{
					Admins: []string{"bob"},
				},
			},
		},
		{
			Name: "MaxNumberUserAdmins fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:    false,
				UserAdminsAllowed:   true,
				MaxNumberUserAdmins: 1,
				TeamAdminsAllowed:   true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "The number of users with admin permission on this repository is greater than the allowed maximum value.",
				Details: details{
					Admins: []string{"alice", "bob"},
				},
			},
		},
		{
			Name: "MaxNumberUserAdmins pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:    false,
				UserAdminsAllowed:   true,
				MaxNumberUserAdmins: 2,
				TeamAdminsAllowed:   true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: []string{"alice", "bob"},
				},
			},
		},
		{
			Name: "MaxNumberUserAdmins pass 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:    true,
				UserAdminsAllowed:   true,
				MaxNumberUserAdmins: 2,
				TeamAdminsAllowed:   true,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: nil,
				},
			},
		},
		{
			Name: "MaxNumberUserAdmins allowed by an exemption and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:    false,
				UserAdminsAllowed:   false,
				MaxNumberUserAdmins: 1,
				TeamAdminsAllowed:   false,
				Exemptions: []*AdministratorExemption{
					{
						Repo:                "thisrepo",
						OwnerlessAllowed:    false,
						UserAdminsAllowed:   true,
						MaxNumberUserAdmins: 3,
						TeamAdminsAllowed:   false,
					},
				},
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: []string{"alice", "bob"},
				},
			},
		},
		{
			Name: "MaxNumberUserAdmins not llowed by an exemption and fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:    false,
				UserAdminsAllowed:   false,
				MaxNumberUserAdmins: 3,
				TeamAdminsAllowed:   false,
				Exemptions: []*AdministratorExemption{
					{
						Repo:                "thisrepo",
						OwnerlessAllowed:    false,
						UserAdminsAllowed:   true,
						MaxNumberUserAdmins: 2,
						TeamAdminsAllowed:   false,
					},
				},
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
				&github.User{
					Login: &dave,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "The number of users with admin permission on this repository is greater than the allowed maximum value.",
				Details: details{
					Admins: []string{"alice", "bob", "dave"},
				},
			},
		},
		{
			Name: "TeamAdminsAllowed not allowed and fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: false,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Teams are not allowed to be administrators of this repository.\nInstead a team should be added as administrator.",
				Details: details{
					TeamAdmins: []string{"bob"},
				},
			},
		},
		{
			Name: "TeamAdminsAllowed not allowed and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: false,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins: nil,
				},
			},
		},
		{
			Name: "TeamAdminsAllowed not allowed and pass 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: false,
			},
			Repo: RepoConfig{},
			Users: []*github.User{
				&github.User{
					Login: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.User{
					Login: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Admins:     []string{"bob"},
					TeamAdmins: nil,
				},
			},
		},
		{
			Name: "TeamAdminsAllowed allowed and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					TeamAdmins: []string{"bob"},
				},
			},
		},
		{
			Name: "TeamAdminsAllowed allowed and pass 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: true,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					TeamAdmins: nil,
				},
			},
		},
		{
			Name: "TeamAdminsAllowed not allowed but allowed by an exemption and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: false,
				Exemptions: []*AdministratorExemption{
					{
						Repo:              "thisrepo",
						OwnerlessAllowed:  true,
						TeamAdminsAllowed: true,
					},
				},
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					TeamAdmins: []string{"bob"},
				},
			},
		},
		{
			Name: "TeamAdminsAllowed not allowed but allowed by an exemption and pass 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: true,
				TeamAdminsAllowed: false,
				Exemptions: []*AdministratorExemption{
					{
						Repo:             "thisrepo",
						OwnerlessAllowed: true,
						TeamAdmins:       []string{"dave", "bob"},
					},
				},
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					TeamAdmins: []string{"bob"},
				},
			},
		},
		{
			Name: "TeamAdminsAllowed not allowed by by an exemption and fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: false,
				Exemptions: []*AdministratorExemption{
					{
						Repo:              "thisrepo",
						OwnerlessAllowed:  true,
						TeamAdminsAllowed: false,
					},
				},
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Teams are not allowed to be administrators of this repository.\nInstead a user should be added as administrator.",
				Details: details{
					TeamAdmins: []string{"bob"},
				},
			},
		},
		{
			Name: "TeamAdminsAllowed not allowed by by an exemption and fail 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:  true,
				UserAdminsAllowed: false,
				TeamAdminsAllowed: false,
				Exemptions: []*AdministratorExemption{
					{
						Repo:             "thisrepo",
						OwnerlessAllowed: true,
						TeamAdmins:       []string{"dave"},
					},
				},
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Teams are not allowed to be administrators of this repository.\nInstead a user should be added as administrator.",
				Details: details{
					TeamAdmins: []string{"bob"},
				},
			},
		},
		{
			Name: "MaxNumberAdminTeams fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:    false,
				UserAdminsAllowed:   false,
				TeamAdminsAllowed:   true,
				MaxNumberAdminTeams: 1,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "The number of teams with admin permission on this repository is greater than the allowed maximum value.",
				Details: details{
					TeamAdmins: []string{"alice", "bob"},
				},
			},
		},
		{
			Name: "MaxNumberAdminTeams pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:    false,
				UserAdminsAllowed:   false,
				TeamAdminsAllowed:   true,
				MaxNumberAdminTeams: 2,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					TeamAdmins: []string{"alice", "bob"},
				},
			},
		},
		{
			Name: "MaxNumberAdminTeams pass 2",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:    true,
				UserAdminsAllowed:   false,
				TeamAdminsAllowed:   true,
				MaxNumberAdminTeams: 2,
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					TeamAdmins: nil,
				},
			},
		},
		{
			Name: "MaxNumberAdminTeams allowed by an exemption and pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:    false,
				UserAdminsAllowed:   false,
				TeamAdminsAllowed:   true,
				MaxNumberAdminTeams: 1,
				Exemptions: []*AdministratorExemption{
					{
						Repo:                "thisrepo",
						OwnerlessAllowed:    false,
						UserAdminsAllowed:   false,
						TeamAdminsAllowed:   true,
						MaxNumberAdminTeams: 3,
					},
				},
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					TeamAdmins: []string{"alice", "bob"},
				},
			},
		},
		{
			Name: "MaxNumberAdminTeams not allowed by an exemption and fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				OwnerlessAllowed:    false,
				UserAdminsAllowed:   false,
				TeamAdminsAllowed:   true,
				MaxNumberAdminTeams: 3,
				Exemptions: []*AdministratorExemption{
					{
						Repo:                "thisrepo",
						OwnerlessAllowed:    false,
						UserAdminsAllowed:   false,
						TeamAdminsAllowed:   true,
						MaxNumberAdminTeams: 2,
					},
				},
			},
			Repo: RepoConfig{},
			Teams: []*github.Team{
				&github.Team{
					Slug: &alice,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
				&github.Team{
					Slug: &bob,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
				&github.Team{
					Slug: &dave,
					Permissions: map[string]bool{
						"push":  true,
						"admin": true,
					},
				},
			},
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "The number of teams with admin permission on this repository is greater than the allowed maximum value.",
				Details: details{
					TeamAdmins: []string{"alice", "bob", "dave"},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configFetchConfig = func(ctx context.Context, c *github.Client,
				owner, repo, path string, ol config.ConfigLevel, out interface{}) error {
				if repo == "thisrepo" && ol == config.RepoLevel {
					rc := out.(*RepoConfig)
					*rc = test.Repo
				} else if ol == config.OrgLevel {
					oc := out.(*OrgConfig)
					*oc = test.Org
				}
				return nil
			}
			listCollaborators = func(c context.Context, o, r string,
				op *github.ListCollaboratorsOptions) ([]*github.User, *github.Response, error) {
				return test.Users, &github.Response{NextPage: 0}, nil
			}
			configIsEnabled = func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig,
				c *github.Client, owner, repo string) (bool, error) {
				return test.cofigEnabled, nil
			}
			listTeams = func(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.Team, *github.Response, error) {
				return test.Teams, &github.Response{NextPage: 0}, nil
			}
			res, err := check(context.Background(), mockRepos{}, nil, "", "thisrepo")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			c := cmp.Comparer(func(x, y string) bool { return trunc(x, 40) == trunc(y, 40) })
			if diff := cmp.Diff(&test.Exp, res, c); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}

func trunc(s string, n int) string {
	if n >= len(s) {
		return s
	}
	return s[:n]
}
