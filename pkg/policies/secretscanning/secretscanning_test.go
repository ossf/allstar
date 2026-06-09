// Copyright 2026 Allstar Authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package secretscanning

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v84/github"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
)

var get func(context.Context, string, string) (*github.Repository, *github.Response, error)

var edit func(context.Context, string, string, *github.Repository) (*github.Repository, *github.Response, error)

type mockRepos struct{}

func (m mockRepos) Get(ctx context.Context, o string, r string) (
	*github.Repository, *github.Response, error,
) {
	return get(ctx, o, r)
}

func (m mockRepos) Edit(ctx context.Context, o, r string, repo *github.Repository) (
	*github.Repository, *github.Response, error,
) {
	return edit(ctx, o, r, repo)
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
				Action: github.Ptr("log"),
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
				Action: github.Ptr("log"),
			},
			Repo: RepoConfig{
				Action: github.Ptr("email"),
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
				Action: github.Ptr("log"),
			},
			Repo: RepoConfig{
				Action: github.Ptr("email"),
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
				owner, repo, path string, ol config.ConfigLevel, out interface{},
			) error {
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

			s := SecretScanning(true)
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
		Name         string
		Org          OrgConfig
		Repo         RepoConfig
		Status       *string
		Available    bool
		GetForbidden bool
		cofigEnabled bool
		Exp          policydef.Result
	}{
		{
			Name:         "Pass",
			Org:          OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			Repo:         RepoConfig{},
			Status:       github.Ptr(secretEnabled),
			Available:    true,
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Available: true,
					Status:    secretEnabled,
					URL:       securityAnalysisURL("", "thisrepo"),
				},
			},
		},
		{
			Name:         "Fail",
			Org:          OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			Repo:         RepoConfig{},
			Status:       github.Ptr(secretDisabled),
			Available:    true,
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Secret scanning not enabled.\nGitHub secret scanning checks repositories",
				Details: details{
					Available: true,
					Status:    secretDisabled,
					URL:       securityAnalysisURL("", "thisrepo"),
				},
			},
		},
		{
			Name:         "Unavailable",
			Org:          OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			Repo:         RepoConfig{},
			Available:    false,
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Available: false,
					Status:    secretUnavailable,
					URL:       securityAnalysisURL("", "thisrepo"),
				},
			},
		},
		{
			Name:         "ForbiddenIsUnavailable",
			Org:          OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			Repo:         RepoConfig{},
			GetForbidden: true,
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Available: false,
					Status:    secretUnavailable,
					URL:       securityAnalysisURL("", "thisrepo"),
				},
			},
		},
		{
			Name:         "UnknownStatusPasses",
			Org:          OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			Repo:         RepoConfig{},
			Status:       github.Ptr("unsupported"),
			Available:    true,
			cofigEnabled: true,
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Available: true,
					Status:    "unsupported",
					URL:       securityAnalysisURL("", "thisrepo"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configFetchConfig = func(ctx context.Context, c *github.Client,
				owner, repo, path string, ol config.ConfigLevel, out interface{},
			) error {
				if repo == "thisrepo" && ol == config.RepoLevel {
					rc := out.(*RepoConfig)
					*rc = test.Repo
				} else if ol == config.OrgLevel {
					oc := out.(*OrgConfig)
					*oc = test.Org
				}
				return nil
			}
			get = func(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error) {
				if test.GetForbidden {
					return nil, &github.Response{Response: &http.Response{StatusCode: http.StatusForbidden}}, errors.New("forbidden")
				}
				r := &github.Repository{}
				if test.Available {
					r.SecurityAndAnalysis = &github.SecurityAndAnalysis{
						SecretScanning: &github.SecretScanning{
							Status: test.Status,
						},
					}
				}
				return r, nil, nil
			}
			configIsEnabled = func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig,
				c *github.Client, owner, repo string,
			) (bool, error) {
				return test.cofigEnabled, nil
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

func TestFix(t *testing.T) {
	tests := []struct {
		Name          string
		Org           OrgConfig
		Status        *string
		Available     bool
		GetForbidden  bool
		EditForbidden bool
		cofigEnabled  bool
		ExpEdit       bool
	}{
		{
			Name:         "EnabledNoop",
			Org:          OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			Status:       github.Ptr(secretEnabled),
			Available:    true,
			cofigEnabled: true,
		},
		{
			Name:         "DisabledEnables",
			Org:          OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			Status:       github.Ptr(secretDisabled),
			Available:    true,
			cofigEnabled: true,
			ExpEdit:      true,
		},
		{
			Name:         "UnavailableNoop",
			Org:          OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			Available:    false,
			cofigEnabled: true,
		},
		{
			Name:         "ForbiddenNoop",
			Org:          OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			GetForbidden: true,
			cofigEnabled: true,
		},
		{
			Name:         "PolicyDisabledNoop",
			Org:          OrgConfig{},
			Status:       github.Ptr(secretDisabled),
			Available:    true,
			cofigEnabled: false,
		},
		{
			Name:          "EditForbiddenNoError",
			Org:           OrgConfig{OptConfig: config.OrgOptConfig{OptOutStrategy: true}},
			Status:        github.Ptr(secretDisabled),
			Available:     true,
			EditForbidden: true,
			cofigEnabled:  true,
			ExpEdit:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configFetchConfig = func(ctx context.Context, c *github.Client,
				owner, repo, path string, ol config.ConfigLevel, out interface{},
			) error {
				if ol == config.OrgLevel {
					oc := out.(*OrgConfig)
					*oc = test.Org
				}
				return nil
			}
			get = func(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error) {
				if test.GetForbidden {
					return nil, &github.Response{Response: &http.Response{StatusCode: http.StatusForbidden}}, errors.New("forbidden")
				}
				r := &github.Repository{}
				if test.Available {
					r.SecurityAndAnalysis = &github.SecurityAndAnalysis{
						SecretScanning: &github.SecretScanning{
							Status: test.Status,
						},
					}
				}
				return r, nil, nil
			}
			var gotEdit bool
			edit = func(ctx context.Context, owner, repo string, r *github.Repository) (*github.Repository, *github.Response, error) {
				gotEdit = true
				if r.SecurityAndAnalysis == nil ||
					r.SecurityAndAnalysis.SecretScanning == nil ||
					r.SecurityAndAnalysis.SecretScanning.GetStatus() != secretEnabled {
					t.Fatalf("Edit() called without enabling secret scanning: %+v", r)
				}
				if test.EditForbidden {
					return nil, &github.Response{Response: &http.Response{StatusCode: http.StatusForbidden}}, errors.New("forbidden")
				}
				return r, nil, nil
			}
			configIsEnabled = func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig,
				c *github.Client, owner, repo string,
			) (bool, error) {
				return test.cofigEnabled, nil
			}

			err := fix(context.Background(), mockRepos{}, nil, "", "thisrepo")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if gotEdit != test.ExpEdit {
				t.Errorf("Unexpected edit call. want %v, got %v", test.ExpEdit, gotEdit)
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
