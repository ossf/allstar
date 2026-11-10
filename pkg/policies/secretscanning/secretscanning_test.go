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
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v84/github"

	"github.com/ossf/allstar/internal/testutil"
	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
)

type mockRepos struct {
	repository *github.Repository
	edited     *github.Repository
}

func (m *mockRepos) Get(context.Context, string, string) (*github.Repository, *github.Response, error) {
	return m.repository, nil, nil
}

func (m *mockRepos) Edit(_ context.Context, _, _ string, repository *github.Repository) (*github.Repository, *github.Response, error) {
	m.edited = repository
	return repository, nil, nil
}

//nolint:paralleltest // Replaces package-level configuration mocks.
func TestConfigPrecedence(t *testing.T) {
	originalFetch := configFetchConfig
	t.Cleanup(func() { configFetchConfig = originalFetch })

	tests := []struct {
		name     string
		org      OrgConfig
		orgRepo  RepoConfig
		repo     RepoConfig
		expected string
	}{
		{
			name:     "org config",
			org:      OrgConfig{Action: "issue"},
			expected: "issue",
		},
		{
			name:     "org repo override",
			org:      OrgConfig{Action: "issue"},
			orgRepo:  RepoConfig{Action: github.Ptr("log")},
			expected: "log",
		},
		{
			name:     "repo override",
			org:      OrgConfig{Action: "issue"},
			orgRepo:  RepoConfig{Action: github.Ptr("log")},
			repo:     RepoConfig{Action: github.Ptr("email")},
			expected: "email",
		},
		{
			name:     "repo override disabled",
			org:      OrgConfig{Action: "issue", OptConfig: config.OrgOptConfig{DisableRepoOverride: true}},
			orgRepo:  RepoConfig{Action: github.Ptr("log")},
			repo:     RepoConfig{Action: github.Ptr("email")},
			expected: "log",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configFetchConfig = func(_ context.Context, _ *github.Client, _, _ string, _ string, level config.ConfigLevel, out interface{}) error {
				switch level {
				case config.OrgLevel:
					*testutil.RequireType[*OrgConfig](t, out) = test.org
				case config.OrgRepoLevel:
					*testutil.RequireType[*RepoConfig](t, out) = test.orgRepo
				case config.RepoLevel:
					*testutil.RequireType[*RepoConfig](t, out) = test.repo
				}
				return nil
			}

			action := SecretScanning(true).GetAction(context.Background(), nil, "owner", "repo")
			if action != test.expected {
				t.Fatalf("action = %q, want %q", action, test.expected)
			}
		})
	}
}

//nolint:paralleltest // Replaces package-level configuration mocks.
func TestCheck(t *testing.T) {
	originalFetch := configFetchConfig
	originalEnabled := configIsEnabled
	t.Cleanup(func() {
		configFetchConfig = originalFetch
		configIsEnabled = originalEnabled
	})

	tests := []struct {
		name       string
		enabled    bool
		repository *github.Repository
		expected   policydef.Result
	}{
		{
			name:     "policy disabled without status",
			enabled:  false,
			expected: policydef.Result{Enabled: false},
		},
		{
			name:       "secret scanning enabled",
			enabled:    true,
			repository: repositoryWithStatus("enabled"),
			expected: policydef.Result{
				Enabled: true,
				Pass:    true,
				Details: details{Status: "enabled", Available: true},
			},
		},
		{
			name:       "secret scanning disabled",
			enabled:    true,
			repository: repositoryWithStatus("disabled"),
			expected: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "Secret scanning not enabled.\n" + fmt.Sprintf(notifyText, "owner", "repo"),
				Details:    details{Status: "disabled", Available: true},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configFetchConfig = func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error {
				return nil
			}
			configIsEnabled = func(context.Context, config.OrgOptConfig, config.RepoOptConfig, config.RepoOptConfig, *github.Client, string, string) (bool, error) {
				return test.enabled, nil
			}

			result, err := check(context.Background(), &mockRepos{repository: test.repository}, nil, "owner", "repo")
			if err != nil {
				t.Fatalf("check() error = %v", err)
			}
			if diff := cmp.Diff(&test.expected, result); diff != "" {
				t.Fatalf("check() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

//nolint:paralleltest // Replaces package-level configuration mocks.
func TestCheckUnavailableStatus(t *testing.T) {
	originalFetch := configFetchConfig
	originalEnabled := configIsEnabled
	t.Cleanup(func() {
		configFetchConfig = originalFetch
		configIsEnabled = originalEnabled
	})
	configFetchConfig = func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error {
		return nil
	}
	configIsEnabled = func(context.Context, config.OrgOptConfig, config.RepoOptConfig, config.RepoOptConfig, *github.Client, string, string) (bool, error) {
		return true, nil
	}

	tests := []struct {
		name string
		body string
	}{
		{name: "nil repository", body: `null`},
		{name: "missing security settings", body: `{}`},
		{name: "null security settings", body: `{"security_and_analysis":null}`},
		{name: "missing secret scanning", body: `{"security_and_analysis":{}}`},
		{name: "null secret scanning", body: `{"security_and_analysis":{"secret_scanning":null}}`},
		{name: "missing status", body: `{"security_and_analysis":{"secret_scanning":{}}}`},
		{name: "null status", body: `{"security_and_analysis":{"secret_scanning":{"status":null}}}`},
		{name: "empty status", body: `{"security_and_analysis":{"secret_scanning":{"status":""}}}`},
		{name: "unknown status", body: `{"security_and_analysis":{"secret_scanning":{"status":"unknown"}}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var repository *github.Repository
			if err := json.Unmarshal([]byte(test.body), &repository); err != nil {
				t.Fatal(err)
			}
			result, err := check(context.Background(), &mockRepos{repository: repository}, nil, "owner", "repo")
			if !errors.Is(err, errStatusUnavailable) {
				t.Fatalf("check() error = %v, want unavailable status", err)
			}
			if result != nil {
				t.Fatalf("check() result = %+v, want nil for unavailable status", result)
			}
		})
	}
}

func TestFixEnablesSecretScanning(t *testing.T) {
	t.Parallel()

	repositories := &mockRepos{}

	err := fix(context.Background(), repositories, "owner", "repo")
	if err != nil {
		t.Fatalf("Fix() error = %v", err)
	}
	if repositories.edited == nil || repositories.edited.SecurityAndAnalysis == nil || repositories.edited.SecurityAndAnalysis.SecretScanning == nil {
		t.Fatal("Fix() did not send secret scanning configuration")
	}
	if got := repositories.edited.SecurityAndAnalysis.SecretScanning.GetStatus(); got != "enabled" {
		t.Fatalf("Fix() status = %q, want enabled", got)
	}
}

func repositoryWithStatus(status string) *github.Repository {
	return &github.Repository{
		SecurityAndAnalysis: &github.SecurityAndAnalysis{
			SecretScanning: &github.SecretScanning{Status: github.Ptr(status)},
		},
	}
}
