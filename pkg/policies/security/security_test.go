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

package security

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v43/github"
	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
)

var query func(context.Context, interface{}, map[string]interface{}) error

type mockClient struct{}

func (m mockClient) Query(ctx context.Context, q interface{}, v map[string]interface{}) error {
	return query(ctx, q, v)
}

func TestCheck(t *testing.T) {
	tests := []struct {
		Name         string
		Org          OrgConfig
		Repo         RepoConfig
		SecEnabled   bool
		cofigEnabled bool
		Exp          policydef.Result
	}{
		{
			Name:         "NotEnabled",
			Org:          OrgConfig{},
			Repo:         RepoConfig{},
			SecEnabled:   true,
			cofigEnabled: false,
			Exp: policydef.Result{
				Enabled:    false,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Enabled: true,
					URL:     "",
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
			Repo:         RepoConfig{},
			SecEnabled:   true,
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Enabled: true,
					URL:     "",
				},
			},
		},
		{
			Name: "Fail",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
			},
			Repo:         RepoConfig{},
			SecEnabled:   false,
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Security policy not enabled.\nA SECURITY.md file can give users information about what constitutes a vulnerability",
				Details: details{
					Enabled: false,
					URL:     "",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configFetchConfig = func(ctx context.Context, c *github.Client,
				owner string, repo string, path string, ol bool, out interface{}) error {
				if repo == "thisrepo" {
					rc := out.(*RepoConfig)
					*rc = test.Repo
				} else {
					oc := out.(*OrgConfig)
					*oc = test.Org
				}
				return nil
			}
			query = func(ctx context.Context, q interface{}, v map[string]interface{}) error {
				qc, ok := q.(*struct {
					Repository struct {
						SecurityPolicyUrl       string
						IsSecurityPolicyEnabled bool
					} `graphql:"repository(owner: $owner, name: $name)"`
				})
				if !ok {
					t.Errorf("Query() called with unexpected query structure.")
				}
				qc.Repository.IsSecurityPolicyEnabled = test.SecEnabled
				return nil
			}
			configIsEnabled = func(ctx context.Context, o config.OrgOptConfig, r config.RepoOptConfig,
				c *github.Client, owner, repo string) (bool, error) {
				return test.cofigEnabled, nil
			}
			res, err := check(context.Background(), nil, mockClient{}, "", "thisrepo")
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
