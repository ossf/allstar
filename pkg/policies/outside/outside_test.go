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

package outside

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v39/github"
	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
)

var listCollaborators func(context.Context, string, string,
	*github.ListCollaboratorsOptions) ([]*github.User, *github.Response, error)

type mockRepos struct{}

func (m mockRepos) ListCollaborators(ctx context.Context, o, r string,
	op *github.ListCollaboratorsOptions) ([]*github.User, *github.Response, error) {
	return listCollaborators(ctx, o, r, op)
}

func TestCheck(t *testing.T) {
	bob := "bob"
	alice := "alice"
	tests := []struct {
		Name  string
		Org   OrgConfig
		Repo  RepoConfig
		Users []*github.User
		Exp   policydef.Result
	}{
		{
			Name: "NotEnabled",
			Org: OrgConfig{
				PushAllowed: true,
			},
			Repo:  RepoConfig{},
			Users: nil,
			Exp: policydef.Result{
				Enabled:    false,
				Pass:       true,
				NotifyText: "",
				Details:    details{},
			},
		},
		{
			Name: "NoOC",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				PushAllowed: true,
			},
			Repo:  RepoConfig{},
			Users: nil,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details:    details{},
			},
		},
		{
			Name: "Pushers allowed",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				PushAllowed: true,
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
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					OutsidePushCount: 2,
					OutsidePushers:   []string{"alice", "bob"},
				},
			},
		},
		{
			Name: "Admin blocked",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				PushAllowed: true,
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
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Found 1 outside collaborators with admin access.\nThis policy requires all users with admin access to be members of the organisation.",
				Details: details{
					OutsidePushCount:  2,
					OutsidePushers:    []string{"alice", "bob"},
					OutsideAdminCount: 1,
					OutsideAdmins:     []string{"bob"},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configFetchConfig = func(ctx context.Context, c *github.Client,
				owner string, repo string, path string, out interface{}) error {
				if repo == "thisrepo" {
					rc := out.(*RepoConfig)
					*rc = test.Repo
				} else {
					oc := out.(*OrgConfig)
					*oc = test.Org
				}
				return nil
			}
			listCollaborators = func(c context.Context, o, r string,
				op *github.ListCollaboratorsOptions) ([]*github.User, *github.Response, error) {
				return test.Users, &github.Response{NextPage: 0}, nil
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
