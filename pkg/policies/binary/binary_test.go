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
	"github.com/google/go-github/v39/github"
	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/configdef"
)

func TestGetOrgActionConfig(t *testing.T) {
	tests := []struct {
		Name       string
		Org        OrgConfig
		Repo       RepoConfig
		Exp        configdef.OrgActionConfig
	}{
		{
			Name:       "Pass",
			Org:        OrgConfig{},
			Repo:       RepoConfig{},
			Exp: configdef.OrgActionConfig{
				IssueLabel: operator.GitHubIssueLabel,
				IssueFooter: operator.GitHubIssueFooter,
			},
		},
		{
			Name:       "OrgCustomIssueLabel",
			Org:        OrgConfig{
				ActionConfig: config.OrgActionConfig{
					IssueLabel: "customlabel",
				},
			},
			Repo:       RepoConfig{},
			Exp: configdef.OrgActionConfig{
				IssueLabel: "customlabel",
				IssueFooter: operator.GitHubIssueFooter,
			},
		},
		{
			Name:       "RepoCustomIssueLabel",
			Org:        OrgConfig{},
			Repo:       RepoConfig{
				ActionConfig: config.OrgActionConfig{
					IssueLabel: "customlabel",
				},
			},
			Exp: configdef.OrgActionConfig{
				IssueLabel: "customlabel",
				IssueFooter: operator.GitHubIssueFooter,
			},
		},
		{
			Name:       "OrgCustomIssueFooter",
			Org:        OrgConfig{
				ActionConfig: config.OrgActionConfig{
					IssueFooter: "customfooter",
				},
			},
			Repo:       RepoConfig{},
			Exp: configdef.OrgActionConfig{
				IssueLabel: operator.GitHubIssueLabel,
				IssueFooter: "customfooter",
			},
		},
		{
			Name:       "RepoCustomIssueFooter",
			Org:        OrgConfig{},
			Repo:       RepoConfig{
				ActionConfig: config.OrgActionConfig{
					IssueFooter: "customfooter",
				},
			},
			Exp: configdef.OrgActionConfig{
				IssueLabel: operator.GitHubIssueLabel,
				IssueFooter: "customfooter",
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
			got := getOrgActionConfig(context.Background(), nil, "", "thisrepo")
			if diff := cmp.Diff(test.Exp, got); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}
