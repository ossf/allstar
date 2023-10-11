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

package catalog

import (
	"context"
	"net/http"
	"testing"

	"github.com/contentful/allstar/pkg/config"
	"github.com/contentful/allstar/pkg/policydef"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v50/github"
)

type mockRepos struct{}
type MockGhClient struct{}

var catalogExists func(ctx context.Context, c *github.Client, owner, repo string) (bool, error)

func (m mockRepos) catalogExists(ctx context.Context, c *github.Client, owner, repo string) (bool, error) {
	return catalogExists(ctx, c, owner, repo)
}
func (m MockGhClient) Get(i int64) (*github.Client, error) {
	return github.NewClient(&http.Client{}), nil
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
				Action: "issue",
			},
			OrgRepo:   RepoConfig{},
			Repo:      RepoConfig{},
			ExpAction: "issue",
			Exp: mergedConfig{
				Action: "issue",
			},
		},
		{
			Name: "OrgRepoOverOrg",
			Org: OrgConfig{
				Action: "issue",
			},
			OrgRepo: RepoConfig{
				Action: github.String("log"),
			},
			Repo:      RepoConfig{},
			ExpAction: "log",
			Exp: mergedConfig{
				Action: "log",
			},
		},
		{
			Name: "RepoOverAllOrg",
			Org: OrgConfig{
				Action: "issue",
			},
			OrgRepo: RepoConfig{
				Action: github.String("log"),
			},
			Repo: RepoConfig{
				Action: github.String("email"),
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
				Action: github.String("log"),
			},
			Repo: RepoConfig{
				Action: github.String("email"),
			},
			ExpAction: "log",
			Exp: mergedConfig{
				Action: "log",
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

			s := Catalog(true)
			ctx := context.Background()

			action := s.GetAction(ctx, nil, "", "thisrepo")
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
	tests := []struct {
		Name          string
		Org           OrgConfig
		Repo          RepoConfig
		configEnabled bool
		Exp           policydef.Result
		ErrorCount    int
	}{
		{
			Name:          "FailNotPresent",
			Org:           OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			Repo:          RepoConfig{},
			configEnabled: false,
			ErrorCount:    0,
			Exp: policydef.Result{
				Enabled:    false,
				Pass:       false,
				NotifyText: "catalog-info file not present.\n" + notifyText,
				Details: details{
					CatalogFound: false,
				},
			},
		},
		{
			Name: "Pass",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
			},
			Repo:          RepoConfig{},
			configEnabled: true,
			ErrorCount:    0,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					CatalogFound: true,
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
			configIsEnabled = func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig,
				c *github.Client, owner, repo string) (bool, error) {
				return test.configEnabled, nil
			}
			catalogExists = func(ctx context.Context, c *github.Client, owner, repo string) (bool, error) {

				path := "/contents/catalog-info.yaml"
				opts := &github.RepositoryContentGetOptions{
					Ref: "master",
				}

				//GetContentsURL returns the ContentsURL field if it's non-nil, zero value otherwise.
				_, _, _, err := c.Repositories.GetContents(ctx, owner, repo, path, opts)
				if err != nil {
					t.Fatalf("Unexpected error checking for file: %v", err)
					return false, nil
				}

				return true, nil
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
