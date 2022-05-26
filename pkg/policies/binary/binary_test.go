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

package binary

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v43/github"
	"github.com/ossf/allstar/pkg/config"
)

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

			b := Binary(true)
			ctx := context.Background()

			action := b.GetAction(ctx, nil, "", "thisrepo")
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
