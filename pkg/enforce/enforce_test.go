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
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v43/github"
	"github.com/ossf/allstar/pkg/ghclients"
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
		ExpEnforceResults EnforceResults
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
			ExpEnforceResults: EnforceResults{
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
			ExpEnforceResults: EnforceResults{
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
			ExpEnforceResults: EnforceResults{
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
			ExpEnforceResults: EnforceResults{
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
			ExpEnforceResults: EnforceResults{
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
			ExpEnforceResults: EnforceResults{},
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fixCalled = false
			ensureCalled = false
			closeCalled = false
			policy1Results = test.Res
			action = test.Action
			enforceResults, err := RunPolicies(context.Background(), nil, "", repo, true)
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

type MockGhClients struct{}

func (m MockGhClients) Get(i int64) (*github.Client, error) {
	return github.NewClient(&http.Client{Transport: http.DefaultTransport}), nil
}

func (m MockGhClients) LogCacheSize() {}

func TestEnforceAll(t *testing.T) {
	policiesGetPolicies = func() []policydef.Policy {
		return []policydef.Policy{
			pol{},
			pol2{},
		}
	}
	issueEnsure = func(ctx context.Context, c *github.Client, owner, repo, policy, text string) error {
		return nil
	}
	issueClose = func(ctx context.Context, c *github.Client, owner, repo, policy string) error {
		return nil
	}
	getAppInstallations = func(ctx context.Context, ghc ghclients.GhClientsInterface) ([]*github.Installation, error) {
		var insts []*github.Installation
		appID := int64(123456)
		inst := &github.Installation{
			ID: &appID,
		}
		insts = append(insts, inst)
		return insts, nil
	}
	getAppInstallationRepos = func(ctx context.Context, ghc ghclients.GhClientsInterface, ic *github.Client) ([]*github.Repository, *github.Response, error) {
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
	isBotEnabled = func(ctx context.Context, c *github.Client, owner, repo string) bool {
		return true
	}

	mockGhc := &MockGhClients{}
	action = "log"

	type EnforceTest struct {
		Name           string
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
			policy1Results = test.Policy1Results
			policy2Results = test.Policy2Results
			enforceAllResults, err := EnforceAll(context.Background(), mockGhc)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff(test.ExpResults, enforceAllResults); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}
