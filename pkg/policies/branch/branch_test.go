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

package branch

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/go-github/v35/github"
	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
)

var get func(context.Context, string, string) (*github.Repository,
	*github.Response, error)
var listBranches func(context.Context, string, string,
	*github.BranchListOptions) ([]*github.Branch, *github.Response, error)
var getBranchProtection func(context.Context, string, string, string) (
	*github.Protection, *github.Response, error)

type mockRepos struct{}

func (m mockRepos) Get(ctx context.Context, o string, r string) (
	*github.Repository, *github.Response, error) {
	return get(ctx, o, r)
}

func (m mockRepos) ListBranches(ctx context.Context, o string, r string,
	op *github.BranchListOptions) ([]*github.Branch, *github.Response, error) {
	return listBranches(ctx, o, r, op)
}

func (m mockRepos) GetBranchProtection(ctx context.Context, o string, r string,
	b string) (*github.Protection, *github.Response, error) {
	return getBranchProtection(ctx, o, r, b)
}

func TestCheck(t *testing.T) {
	one := 1
	fal := false
	tests := []struct {
		Name string
		Org  OrgConfig
		Repo RepoConfig
		Prot map[string]github.Protection
		Exp  policydef.Result
	}{
		{
			Name: "CatchBlockForce",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				EnforceDefault:  true,
				RequireApproval: true,
				ApprovalCount:   1,
				DismissStale:    true,
				BlockForce:      true,
			},
			Repo: RepoConfig{},
			Prot: map[string]github.Protection{
				"main": github.Protection{
					RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
						DismissStaleReviews:          true,
						RequiredApprovingReviewCount: 5,
					},
					AllowForcePushes: &github.AllowForcePushes{
						Enabled: true,
					},
				},
			},
			Exp: policydef.Result{
				Pass:       false,
				NotifyText: "Block force push not configured for branch main\n",
				Details: map[string]details{
					"main": details{
						PRReviews:    true,
						NumReviews:   5,
						DismissStale: true,
						BlockForce:   false,
					},
				},
			},
		},
		{
			Name: "CatchReleaseBranch",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				EnforceDefault:  true,
				RequireApproval: true,
				ApprovalCount:   1,
				DismissStale:    true,
				BlockForce:      true,
			},
			Repo: RepoConfig{
				EnforceBranches: []string{"release"},
			},
			Prot: map[string]github.Protection{
				"main": github.Protection{
					RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
						DismissStaleReviews:          true,
						RequiredApprovingReviewCount: 2,
					},
				},
				"release": github.Protection{},
			},
			Exp: policydef.Result{
				Pass:       false,
				NotifyText: "PR Approvals not configured for branch release\n",
				Details: map[string]details{
					"main": details{
						PRReviews:    true,
						NumReviews:   2,
						DismissStale: true,
						BlockForce:   true,
					},
					"release": details{
						PRReviews:    false,
						NumReviews:   0,
						DismissStale: false,
						BlockForce:   true,
					},
				},
			},
		},
		{
			Name: "RepoOverride",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				EnforceDefault:  true,
				RequireApproval: true,
				ApprovalCount:   2,
				DismissStale:    true,
				BlockForce:      true,
			},
			Repo: RepoConfig{
				ApprovalCount: &one,
				DismissStale:  &fal,
			},
			Prot: map[string]github.Protection{
				"main": github.Protection{
					RequiredPullRequestReviews: &github.PullRequestReviewsEnforcement{
						DismissStaleReviews:          false,
						RequiredApprovingReviewCount: 1,
					},
				},
			},
			Exp: policydef.Result{
				Pass:       true,
				NotifyText: "",
				Details: map[string]details{
					"main": details{
						PRReviews:    true,
						NumReviews:   1,
						DismissStale: false,
						BlockForce:   true,
					},
				},
			},
		},
		{
			Name: "NoProtection",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
				EnforceDefault:  true,
				RequireApproval: true,
				ApprovalCount:   2,
				DismissStale:    true,
				BlockForce:      true,
			},
			Repo: RepoConfig{},
			Prot: map[string]github.Protection{},
			Exp: policydef.Result{
				Pass:       false,
				NotifyText: "No protection found for branch main\n",
				Details: map[string]details{
					"main": details{
						PRReviews:    false,
						NumReviews:   0,
						DismissStale: false,
						BlockForce:   false,
					},
				},
			},
		},
	}

	get = func(context.Context, string, string) (*github.Repository,
		*github.Response, error) {
		b := "main"
		return &github.Repository{
			DefaultBranch: &b,
		}, nil, nil
	}
	listBranches = func(context.Context, string, string,
		*github.BranchListOptions) ([]*github.Branch, *github.Response, error) {
		return []*github.Branch{
			&github.Branch{},
		}, &github.Response{NextPage: 0}, nil
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
			getBranchProtection = func(ctx context.Context, o string, r string,
				b string) (*github.Protection, *github.Response, error) {
				p, ok := test.Prot[b]
				if ok {
					return &p, nil, nil
				} else {
					return nil, &github.Response{
						Response: &http.Response{
							StatusCode: http.StatusNotFound,
						},
					}, errors.New("404")
				}
			}
			res, err := check(context.Background(), mockRepos{}, nil, "", "thisrepo")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !reflect.DeepEqual(res, &test.Exp) {
				t.Errorf("Unexpected results. Got: %v, Expect: %v", res, &test.Exp)
			}
		})
	}
	t.Run("Emptyrepo", func(t *testing.T) {
		listBranches = func(context.Context, string, string,
			*github.BranchListOptions) ([]*github.Branch, *github.Response, error) {
			return []*github.Branch{}, &github.Response{NextPage: 0}, nil
		}
		res, err := check(context.Background(), mockRepos{}, nil, "", "thisrepo")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expect := &policydef.Result{
			Pass:       true,
			NotifyText: "No branches to protect",
		}
		if !reflect.DeepEqual(res, expect) {
			t.Errorf("Unexpected results. Got: %v, Expect: %v", res, expect)
		}
	})
}
