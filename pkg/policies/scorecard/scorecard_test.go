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

package scorecard

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v84/github"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/scorecard"

	"github.com/ossf/scorecard/v5/checker"
	"github.com/ossf/scorecard/v5/clients"
	sc "github.com/ossf/scorecard/v5/pkg/scorecard"
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
		{
			Name: "UploadDefaultDisabled",
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
			Name: "UploadEnabledAtOrg",
			Org: OrgConfig{
				Action: "issue",
				Upload: UploadConfig{SARIF: true},
			},
			OrgRepo:   RepoConfig{},
			Repo:      RepoConfig{},
			ExpAction: "issue",
			Exp: mergedConfig{
				Action: "issue",
				Upload: UploadConfig{SARIF: true},
			},
		},
		{
			Name: "UploadOrgRepoOverridesOrg",
			Org: OrgConfig{
				Action: "issue",
				Upload: UploadConfig{SARIF: true},
			},
			OrgRepo: RepoConfig{
				Upload: &UploadConfig{SARIF: false},
			},
			Repo:      RepoConfig{},
			ExpAction: "issue",
			Exp: mergedConfig{
				Action: "issue",
				Upload: UploadConfig{SARIF: false},
			},
		},
		{
			Name: "UploadRepoOverridesAll",
			Org: OrgConfig{
				Action: "issue",
				Upload: UploadConfig{SARIF: false},
			},
			OrgRepo: RepoConfig{},
			Repo: RepoConfig{
				Upload: &UploadConfig{SARIF: true},
			},
			ExpAction: "issue",
			Exp: mergedConfig{
				Action: "issue",
				Upload: UploadConfig{SARIF: true},
			},
		},
		{
			Name: "UploadRepoDisallowed",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					DisableRepoOverride: true,
				},
				Action: "issue",
				Upload: UploadConfig{SARIF: false},
			},
			OrgRepo: RepoConfig{},
			Repo: RepoConfig{
				Upload: &UploadConfig{SARIF: true},
			},
			ExpAction: "issue",
			Exp: mergedConfig{
				Action: "issue",
				Upload: UploadConfig{SARIF: false},
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

			w := NewScorecard()
			ctx := context.Background()

			action := w.GetAction(ctx, nil, "", "thisrepo")
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
		Name    string
		Org     OrgConfig
		OrgRepo RepoConfig
		Repo    RepoConfig
		Result  checker.CheckResult
		ExpPass bool
	}{
		{
			Name: "Pass",
			Org: OrgConfig{
				Checks:    []string{"test"},
				Threshold: 10,
			},
			OrgRepo: RepoConfig{},
			Repo:    RepoConfig{},
			Result: checker.CheckResult{
				Score: 10,
			},
			ExpPass: true,
		},
		{
			Name: "Fail",
			Org: OrgConfig{
				Checks:    []string{"test"},
				Threshold: 8,
			},
			OrgRepo: RepoConfig{},
			Repo:    RepoConfig{},
			Result: checker.CheckResult{
				Score: 7,
			},
			ExpPass: false,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configFetchConfig = func(ctx context.Context, c *github.Client, owner,
				repo, path string, ol config.ConfigLevel, out interface{},
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
			configIsEnabled = func(ctx context.Context, o config.OrgOptConfig, orc,
				r config.RepoOptConfig, c *github.Client, owner, repo string) (bool,
				error,
			) {
				return true, nil
			}
			scorecardGet = func(ctx context.Context, fullRepo string, local bool,
				tr http.RoundTripper,
			) (*scorecard.ScClient, error) {
				return &scorecard.ScClient{}, nil
			}
			checksAllChecks = checker.CheckNameToFnMap{}
			checksAllChecks["test"] = checker.Check{}
			scRun = func(context.Context, clients.Repo, ...sc.Option) (sc.Result, error) {
				return sc.Result{
					Checks: []checker.CheckResult{test.Result},
				}, nil
			}
			s := NewScorecard()
			res, err := s.Check(context.Background(), github.NewClient(nil), "", "")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if res.Pass != test.ExpPass {
				t.Errorf("Expected pass: %v, got: %v", test.ExpPass, res.Pass)
			}
		})
	}
}

func TestCheckUnknownCheckSkipped(t *testing.T) {
	origConfigFetchConfig := configFetchConfig
	origConfigIsEnabled := configIsEnabled
	origScorecardGet := scorecardGet
	origChecksAllChecks := checksAllChecks
	origScRun := scRun
	t.Cleanup(func() {
		configFetchConfig = origConfigFetchConfig
		configIsEnabled = origConfigIsEnabled
		scorecardGet = origScorecardGet
		checksAllChecks = origChecksAllChecks
		scRun = origScRun
	})

	configFetchConfig = func(_ context.Context, _ *github.Client, _, _, _ string,
		ol config.ConfigLevel, out interface{},
	) error {
		if ol == config.OrgLevel {
			oc := out.(*OrgConfig)
			*oc = OrgConfig{
				Checks:    []string{"nonexistent-check", "test"},
				Threshold: 8,
			}
		}
		return nil
	}
	configIsEnabled = func(_ context.Context, _ config.OrgOptConfig, _,
		_ config.RepoOptConfig, _ *github.Client, _, _ string,
	) (bool, error) {
		return true, nil
	}
	scorecardGet = func(_ context.Context, _ string, _ bool,
		_ http.RoundTripper,
	) (*scorecard.ScClient, error) {
		return &scorecard.ScClient{}, nil
	}
	checksAllChecks = checker.CheckNameToFnMap{"test": {}}
	// "test" returns a score below threshold so that if the implementation
	// regresses to break-on-unknown and never runs "test", pass stays true
	// and the assertion below catches the regression.
	scRun = func(_ context.Context, _ clients.Repo, _ ...sc.Option) (sc.Result, error) {
		return sc.Result{
			Checks: []checker.CheckResult{{Name: "test", Score: 5}},
		}, nil
	}

	s := NewScorecard()
	res, err := s.Check(context.Background(), github.NewClient(nil), "", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// The unknown check is skipped; "test" still runs and scores 5 < threshold 8,
	// so the policy should not pass.
	if res.Pass {
		t.Error("Expected fail — unknown check should be skipped, valid check with low score should still run and fail")
	}
}

func TestCheckPerCheckErrorSkipped(t *testing.T) {
	origConfigFetchConfig := configFetchConfig
	origConfigIsEnabled := configIsEnabled
	origScorecardGet := scorecardGet
	origChecksAllChecks := checksAllChecks
	origScRun := scRun
	t.Cleanup(func() {
		configFetchConfig = origConfigFetchConfig
		configIsEnabled = origConfigIsEnabled
		scorecardGet = origScorecardGet
		checksAllChecks = origChecksAllChecks
		scRun = origScRun
	})

	configFetchConfig = func(_ context.Context, _ *github.Client, _, _, _ string,
		ol config.ConfigLevel, out interface{},
	) error {
		if ol == config.OrgLevel {
			oc := out.(*OrgConfig)
			*oc = OrgConfig{
				Checks:    []string{"erroring", "failing"},
				Threshold: 8,
			}
		}
		return nil
	}
	configIsEnabled = func(_ context.Context, _ config.OrgOptConfig, _,
		_ config.RepoOptConfig, _ *github.Client, _, _ string,
	) (bool, error) {
		return true, nil
	}
	scorecardGet = func(_ context.Context, _ string, _ bool,
		_ http.RoundTripper,
	) (*scorecard.ScClient, error) {
		return &scorecard.ScClient{}, nil
	}
	checksAllChecks = checker.CheckNameToFnMap{"erroring": {}, "failing": {}}
	// "failing" returns a score below threshold so that if the implementation
	// regresses to break-on-error and never processes "failing", pass stays true
	// and the assertion below catches the regression.
	scRun = func(_ context.Context, _ clients.Repo, _ ...sc.Option) (sc.Result, error) {
		return sc.Result{
			Checks: []checker.CheckResult{
				{Name: "erroring", Error: fmt.Errorf("unsupported check")},
				{Name: "failing", Score: 5},
			},
		}, nil
	}

	s := NewScorecard()
	res, err := s.Check(context.Background(), github.NewClient(nil), "", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// The errored check is skipped; "failing" still runs and scores 5 < threshold 8,
	// so the policy should not pass.
	if res.Pass {
		t.Error("Expected fail — errored check should be skipped, failing check with low score should still run and fail")
	}
}
