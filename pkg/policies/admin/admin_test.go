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
	"github.com/google/go-github/v43/github"
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
				Action:           "issue",
				OwnerlessAllowed: true,
			},
			OrgRepo:   RepoConfig{},
			Repo:      RepoConfig{},
			ExpAction: "issue",
			Exp: mergedConfig{
				Action:           "issue",
				OwnerlessAllowed: true,
			},
		},
		{
			Name: "OrgRepoOverOrg",
			Org: OrgConfig{
				Action:           "issue",
				OwnerlessAllowed: true,
			},
			OrgRepo: RepoConfig{
				Action:           github.String("log"),
				OwnerlessAllowed: github.Bool(false),
			},
			Repo:      RepoConfig{},
			ExpAction: "log",
			Exp: mergedConfig{
				Action:           "log",
				OwnerlessAllowed: false,
			},
		},
		{
			Name: "RepoOverAllOrg",
			Org: OrgConfig{
				Action: "issue",
			},
			OrgRepo: RepoConfig{
				Action:           github.String("log"),
				OwnerlessAllowed: github.Bool(true),
			},
			Repo: RepoConfig{
				Action:           github.String("email"),
				OwnerlessAllowed: github.Bool(false),
			},
			ExpAction: "email",
			Exp: mergedConfig{
				Action: "email",
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
				Action:           github.String("log"),
				OwnerlessAllowed: github.Bool(true),
			},
			Repo: RepoConfig{
				Action:           github.String("email"),
				OwnerlessAllowed: github.Bool(false),
			},
			ExpAction: "log",
			Exp: mergedConfig{
				Action:           "log",
				OwnerlessAllowed: true,
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
				OwnerlessAllowed: true,
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
				OwnerlessAllowed: false,
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
				OwnerlessAllowed: false,
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
				OwnerlessAllowed: true,
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
				OwnerlessAllowed: true,
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
				OwnerlessAllowed: false,
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
				OwnerlessAllowed: false,
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
				OwnerlessAllowed: true,
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
				OwnerlessAllowed: true,
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
				OwnerlessAllowed: false,
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
				OwnerlessAllowed: false,
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
				OwnerlessAllowed: false,
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
				OwnerlessAllowed: false,
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
