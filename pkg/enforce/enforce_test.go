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

package enforce

import (
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v59/github"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/policydef"
)

var policy1Results policyRepoResults
var policy2Results policyRepoResults
var action string
var fixCalled bool

type policyRepoResults map[string]policydef.Result

type pol struct{}

func (p pol) Name() string {
	return "Test policy"
}

func (p pol) IsEnabled(ctx context.Context, c *github.Client, owner, repo string) (bool, error) {
	return true, nil
}

func (p pol) Check(ctx context.Context, c *github.Client, owner, repo string) (*policydef.Result, error) {
	policy1Result := policy1Results[repo]
	return &policy1Result, nil
}

func (p pol) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	fixCalled = true
	return nil
}

func (p pol) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	return action
}

type pol2 struct{}

func (p pol2) Name() string {
	return "Test policy2"
}

func (p pol2) IsEnabled(ctx context.Context, c *github.Client, owner, repo string) (bool, error) {
	return true, nil
}

func (p pol2) Check(ctx context.Context, c *github.Client, owner, repo string) (*policydef.Result, error) {
	policy2Result := policy2Results[repo]
	return &policy2Result, nil
}

func (p pol2) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	return nil
}

func (p pol2) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	return action
}

type MockGhClients struct{}

func (m MockGhClients) Get(i int64) (*github.Client, error) {
	return github.NewClient(&http.Client{}), nil
}

func (m MockGhClients) Free(i int64) {}

func TestRunPolicies(t *testing.T) {
	policiesGetPolicies = func() []policydef.Policy {
		return []policydef.Policy{
			pol{},
		}
	}
	ensureCalled := false
	issueEnsure = func(ctx context.Context, c *github.Client, owner, repo, policy, text string) error {
		ensureCalled = true
		return nil
	}
	closeCalled := false
	issueClose = func(ctx context.Context, c *github.Client, owner, repo, policy string) error {
		closeCalled = true
		return nil
	}
	repo := "fake-repo"
	tests := []struct {
		Name              string
		Res               policyRepoResults
		Action            string
		ShouldFix         bool
		ShouldEnsure      bool
		ShouldClose       bool
		ExpEnforceResults EnforceRepoResults
	}{
		{
			Name: "LogOnly",
			Res: policyRepoResults{
				"fake-repo": policydef.Result{Enabled: true, Pass: false},
			},
			Action:       "log",
			ShouldFix:    false,
			ShouldEnsure: false,
			ShouldClose:  false,
			ExpEnforceResults: EnforceRepoResults{
				"Test policy": false,
			},
		},
		{
			Name: "OpenIssue",
			Res: policyRepoResults{
				"fake-repo": policydef.Result{Enabled: true, Pass: false},
			},
			Action:       "issue",
			ShouldFix:    false,
			ShouldEnsure: true,
			ShouldClose:  false,
			ExpEnforceResults: EnforceRepoResults{
				"Test policy": false,
			},
		},
		{
			Name: "CloseIssue",
			Res: policyRepoResults{
				"fake-repo": policydef.Result{Enabled: true, Pass: true},
			},
			Action:       "issue",
			ShouldFix:    false,
			ShouldEnsure: false,
			ShouldClose:  true,
			ExpEnforceResults: EnforceRepoResults{
				"Test policy": true,
			},
		},
		{
			Name: "Fix",
			Res: policyRepoResults{
				"fake-repo": policydef.Result{Enabled: true, Pass: false},
			},
			Action:       "fix",
			ShouldFix:    true,
			ShouldEnsure: false,
			ShouldClose:  false,
			ExpEnforceResults: EnforceRepoResults{
				"Test policy": false,
			},
		},
		{
			Name: "CloseIssueOnFix",
			Res: policyRepoResults{
				"fake-repo": policydef.Result{Enabled: true, Pass: true},
			},
			Action:       "fix",
			ShouldFix:    false,
			ShouldEnsure: false,
			ShouldClose:  true,
			ExpEnforceResults: EnforceRepoResults{
				"Test policy": true,
			},
		},
		{
			Name: "PolicyDisabled",
			Res: policyRepoResults{
				"fake-repo": policydef.Result{Enabled: false, Pass: false},
			},
			Action:            "fix",
			ShouldFix:         false,
			ShouldEnsure:      false,
			ShouldClose:       false,
			ExpEnforceResults: EnforceRepoResults{},
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fixCalled = false
			ensureCalled = false
			closeCalled = false
			policy1Results = test.Res
			action = test.Action

			enforceResults, err := runPoliciesReal(context.Background(), nil, "", repo, true, "")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if test.ShouldFix != fixCalled {
				if test.ShouldFix {
					t.Error("Expected Fix to be called")
				} else {
					t.Error("Fix called unexpectedly.")
				}
			}
			if test.ShouldEnsure != ensureCalled {
				if test.ShouldEnsure {
					t.Error("Expected Ensure to be called")
				} else {
					t.Error("Ensure called unexpectedly.")
				}
			}
			if test.ShouldClose != closeCalled {
				if test.ShouldClose {
					t.Error("Expected Close to be called")
				} else {
					t.Error("Close called unexpectedly.")
				}
			}
			if diff := cmp.Diff(test.ExpEnforceResults, enforceResults); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRunPoliciesOnInstRepos(t *testing.T) {
	configIsBotEnabled = func(ctx context.Context, c *github.Client, owner, repo string) bool {
		return true
	}

	client := github.NewClient(&http.Client{})
	failErr := errors.New("fail")
	fakeOwner := "fake-owner"

	tests := []struct {
		Name           string
		EnforceResults EnforceRepoResults
		ExpResults     EnforceAllResults
		ExpError       error
		ShouldError    bool
	}{
		{
			Name:        "ReturnsExpectedError",
			ShouldError: true,
			ExpError:    failErr,
		},
		{
			Name: "ReturnsExpectedOwner",
			EnforceResults: EnforceRepoResults{
				"Test policy": true,
			},
			ExpResults: EnforceAllResults{},
		},
		{
			Name: "ReturnsExpectedResults",
			EnforceResults: EnforceRepoResults{
				"Test policy": false,
			},
			ExpResults: EnforceAllResults{
				"Test policy": {
					"totalFailed": 1,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			repo1Name := "repo1"
			repos := []*github.Repository{
				{
					Name: &repo1Name,
					Owner: &github.User{
						Login: &fakeOwner,
					},
				},
			}

			runPolicies = func(ctx context.Context, c *github.Client, owner, repo string, enabled bool, specificPolicyArg string) (EnforceRepoResults, error) {
				if test.ShouldError {
					return nil, failErr
				}
				return test.EnforceResults, nil
			}

			instResults, err := runPoliciesOnInstRepos(context.Background(), repos, client, "")
			if test.ExpError != nil && !errors.Is(test.ExpError, err) {
				t.Fatalf("Error %v does not match expected error %v", err, test.ExpError)
			}
			if test.ExpError == nil && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if err == nil {
				if diff := cmp.Diff(test.ExpResults, instResults); diff != "" {
					t.Errorf("Unexpected results. (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestDoNothingOnOptOut(t *testing.T) {
	policiesGetPolicies = func() []policydef.Policy {
		return []policydef.Policy{
			pol{},
		}
	}

	repo := "fake-repo"
	tests := []struct {
		Name              string
		Res               policyRepoResults
		Enabled           bool
		ExpEnforceResults EnforceRepoResults
		doNothingOnOptOut bool
	}{
		{
			Name: "doNothingOnOptOutNotEnabled",
			Res: policyRepoResults{
				"fake-repo": policydef.Result{Enabled: true, Pass: false},
			},
			Enabled: false,
			ExpEnforceResults: EnforceRepoResults{
				"Test policy": false,
			},
			doNothingOnOptOut: false,
		},
		{
			Name: "doNothingOnOptOutEnabled",
			Res: policyRepoResults{
				"fake-repo": policydef.Result{Enabled: true, Pass: false},
			},
			Enabled:           false,
			ExpEnforceResults: EnforceRepoResults{},
			doNothingOnOptOut: true,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			policy1Results = test.Res

			doNothingOnOptOut = test.doNothingOnOptOut
			enforceResults, err := runPoliciesReal(context.Background(), nil, "", repo, test.Enabled, "")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.ExpEnforceResults, enforceResults); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEnforceAll(t *testing.T) {
	policiesGetPolicies = func() []policydef.Policy {
		return []policydef.Policy{
			pol{},
			pol2{},
		}
	}
	getAppInstallations = func(ctx context.Context, ac *github.Client) ([]*github.Installation, error) {
		var insts []*github.Installation
		appID := int64(123456)
		inst := &github.Installation{
			ID: &appID,
		}
		insts = append(insts, inst)
		return insts, nil
	}
	getAppInstallationRepos = func(ctx context.Context, ic *github.Client) ([]*github.Repository, *github.Response, error) {
		var repos []*github.Repository
		repo1Name := "repo1"
		repo2Name := "repo2"
		ownerLogin := "fake-owner"
		newRepos := []*github.Repository{
			{
				Name: &repo1Name,
				Owner: &github.User{
					Login: &ownerLogin,
				},
			},
			{
				Name: &repo2Name,
				Owner: &github.User{
					Login: &ownerLogin,
				},
			},
		}
		repos = append(repos, newRepos...)
		return repos, nil, nil
	}
	configIsBotEnabled = func(ctx context.Context, c *github.Client, owner, repo string) bool {
		return true
	}

	mockGhc := &MockGhClients{}

	// set back to real value to avoid test interference
	runPolicies = runPoliciesReal

	type EnforceTest struct {
		Name           string
		Action         string
		Policy1Results policyRepoResults
		Policy2Results policyRepoResults
		ExpResults     EnforceAllResults
	}
	tests := []EnforceTest{
		{
			Name: "SinglePolicySingleRepoFailed",
			Policy1Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: true},
				"repo2": {Enabled: true, Pass: false},
			},
			Policy2Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: true},
				"repo2": {Enabled: true, Pass: true},
			},
			ExpResults: EnforceAllResults{
				"Test policy": {
					"totalFailed": 1,
				},
			},
			Action: "log",
		},
		{
			Name: "SinglePolicyBothReposFailed",
			Policy1Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: false},
				"repo2": {Enabled: true, Pass: false},
			},
			Policy2Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: true},
				"repo2": {Enabled: true, Pass: true},
			},
			ExpResults: EnforceAllResults{
				"Test policy": {
					"totalFailed": 2,
				},
			},
			Action: "log",
		},
		{
			Name: "BothPolicySingleRepoFailed",
			Policy1Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: true},
				"repo2": {Enabled: true, Pass: false},
			},
			Policy2Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: true},
				"repo2": {Enabled: true, Pass: false},
			},
			ExpResults: EnforceAllResults{
				"Test policy": {
					"totalFailed": 1,
				},
				"Test policy2": {
					"totalFailed": 1,
				},
			},
			Action: "log",
		},
		{
			Name: "BothPoliciesBothReposPassed",
			Policy1Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: true},
				"repo2": {Enabled: true, Pass: true},
			},
			Policy2Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: true},
				"repo2": {Enabled: true, Pass: true},
			},
			ExpResults: EnforceAllResults{},
			Action:     "log",
		},
		{
			Name: "BothPoliciesSingleRepoDisabled",
			Policy1Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: false},
				"repo2": {Enabled: false, Pass: false},
			},
			Policy2Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: false},
				"repo2": {Enabled: false, Pass: false},
			},
			ExpResults: EnforceAllResults{
				"Test policy": {
					"totalFailed": 1,
				},
				"Test policy2": {
					"totalFailed": 1,
				},
			},
			Action: "log",
		},
		{
			Name: "SinglePolicyBothReposDisabled",
			Policy1Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: false},
				"repo2": {Enabled: true, Pass: false},
			},
			Policy2Results: policyRepoResults{
				"repo1": {Enabled: false, Pass: false},
				"repo2": {Enabled: false, Pass: false},
			},
			ExpResults: EnforceAllResults{
				"Test policy": {
					"totalFailed": 2,
				},
			},
			Action: "log",
		},
		{
			Name: "BothPoliciesBothReposFailed",
			Policy1Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: false},
				"repo2": {Enabled: true, Pass: false},
			},
			Policy2Results: policyRepoResults{
				"repo1": {Enabled: true, Pass: false},
				"repo2": {Enabled: true, Pass: false},
			},
			ExpResults: EnforceAllResults{
				"Test policy": {
					"totalFailed": 2,
				},
				"Test policy2": {
					"totalFailed": 2,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			action = test.Action
			policy1Results = test.Policy1Results
			policy2Results = test.Policy2Results

			enforceAllResults, err := EnforceAll(context.Background(), mockGhc, "", "")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff(test.ExpResults, enforceAllResults); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSuspendedEnforce(t *testing.T) {
	var suspended bool
	var gaicalled bool
	getAppInstallations = func(ctx context.Context, ac *github.Client) ([]*github.Installation, error) {
		var insts []*github.Installation
		appID := int64(123456)
		inst := &github.Installation{
			ID: &appID,
		}
		if suspended {
			inst.SuspendedAt = &github.Timestamp{}
		}
		insts = append(insts, inst)
		return insts, nil
	}
	getAppInstallationRepos = func(ctx context.Context, ic *github.Client) ([]*github.Repository, *github.Response, error) {
		gaicalled = true
		return nil, nil, nil
	}
	suspended = false
	gaicalled = false
	if _, err := EnforceAll(context.Background(), &MockGhClients{}, "", ""); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !gaicalled {
		t.Errorf("Expected getAppInstallationRepos() to be called, but wasn't")
	}
	suspended = true
	gaicalled = false
	if _, err := EnforceAll(context.Background(), &MockGhClients{}, "", ""); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if gaicalled {
		t.Errorf("Expected getAppInstallationRepos() to not be called, but was")
	}
}

func injective(s string) int64 {
	if len(s) < 8 { // pad left
		s = strings.Repeat("_", 8-len(s)) + s
	}
	return int64(binary.BigEndian.Uint64([]byte(s)))
}

func TestAllowedRepositories(t *testing.T) {
	tests := []struct {
		desc       string
		orgs       []string
		allowlist  []string
		expected   []string
		disallowed []string
	}{
		{
			desc:      "all explicitly allowed",
			orgs:      []string{"org-1", "org-2"},
			allowlist: []string{"org-1", "org-2"},
			expected:  []string{"org-1", "org-2"},
		},
		{
			desc:      "some allowed (with duplicate installation)",
			orgs:      []string{"org-1", "org-1", "org-2"},
			allowlist: []string{"org-1"},
			expected:  []string{"org-1", "org-1"},
		},
		{
			desc:       "none allowed",
			orgs:       []string{"org-1", "org-1", "org-2"},
			allowlist:  []string{"org-3"},
			expected:   []string{},
			disallowed: []string{"org-1", "org-1", "org-2"},
		},
		{
			desc:      "empty allowlist allows all by default",
			orgs:      []string{"org-1", "org-2", "org-3"},
			allowlist: []string{},
			// allowed: []string{},
			expected: []string{"org-1", "org-2", "org-3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			listInstallations = func(ctx context.Context, ac *github.Client) ([]*github.Installation, error) {
				repos := []*github.Installation{}
				for _, r := range tt.orgs {
					r := r
					i := injective(r)
					repos = append(repos, &github.Installation{Account: &github.User{Login: &r}, ID: &i})
				}
				return repos, nil
			}

			removed := []int64{}
			deleteInstallation = func(ctx context.Context, ic *github.Client, instID int64) (*github.Response, error) {
				removed = append(removed, instID)
				return &github.Response{Response: &http.Response{StatusCode: 200}}, nil
			}

			getAppInstallations = getAppInstallationsReal
			operator.AllowedOrganizations = tt.allowlist
			insts, err := getAppInstallations(context.Background(), &github.Client{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Ensure that all repos expected to be disallowed are also uninstalled
			for _, r := range tt.disallowed {
				i := injective(r)
				var contains bool
				for _, r := range removed {
					if r == i {
						contains = true
					}
				}
				if !contains {
					t.Errorf("expected disallowed repo %s to be removed but wasn't", r)
				}
			}

			// Ensure that returned repos as expected
			rn := []string{}
			for _, r := range insts {
				rn = append(rn, *r.Account.Login)
			}
			if !reflect.DeepEqual(rn, tt.expected) {
				t.Errorf("expected allowed repos %+v got allowed repos %+v", tt.expected, rn)
			}
		})
	}
}
