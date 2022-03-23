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
	"testing"

	"github.com/google/go-github/v43/github"
	"github.com/ossf/allstar/pkg/policydef"
)

var result policydef.Result
var action string
var fixCalled bool

type pol struct{}

func (p pol) Name() string {
	return "Test policy"
}

func (p pol) Check(ctx context.Context, c *github.Client, owner, repo string) (*policydef.Result, error) {
	return &result, nil
}

func (p pol) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	fixCalled = true
	return nil
}

func (p pol) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
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
	tests := []struct {
		Name         string
		Res          policydef.Result
		Action       string
		ShouldFix    bool
		ShouldEnsure bool
		ShouldClose  bool
	}{
		{
			Name:         "LogOnly",
			Res:          policydef.Result{Enabled: true, Pass: false},
			Action:       "log",
			ShouldFix:    false,
			ShouldEnsure: false,
			ShouldClose:  false,
		},
		{
			Name:         "OpenIssue",
			Res:          policydef.Result{Enabled: true, Pass: false},
			Action:       "issue",
			ShouldFix:    false,
			ShouldEnsure: true,
			ShouldClose:  false,
		},
		{
			Name:         "CloseIssue",
			Res:          policydef.Result{Enabled: true, Pass: true},
			Action:       "issue",
			ShouldFix:    false,
			ShouldEnsure: false,
			ShouldClose:  true,
		},
		{
			Name:         "Fix",
			Res:          policydef.Result{Enabled: true, Pass: false},
			Action:       "fix",
			ShouldFix:    true,
			ShouldEnsure: false,
			ShouldClose:  false,
		},
		{
			Name:         "PolicyDisabled",
			Res:          policydef.Result{Enabled: false, Pass: false},
			Action:       "fix",
			ShouldFix:    false,
			ShouldEnsure: false,
			ShouldClose:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fixCalled = false
			ensureCalled = false
			closeCalled = false
			result = test.Res
			action = test.Action
			err := RunPolicies(context.Background(), nil, "", "", true)
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
		})
	}
}

func TestEnforceAll(t *testing.T) {
	t.Skip("Testing EnforceAll looks tricky, TODO")
}
