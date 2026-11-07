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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v84/github"

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

func (m *mockRepos) Edit(_ context.Context, _ string, _ string, repository *github.Repository) (*github.Repository, *github.Response, error) {
	m.edited = repository
	return repository, nil, nil
}

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
					*out.(*OrgConfig) = test.org
				case config.OrgRepoLevel:
					*out.(*RepoConfig) = test.orgRepo
				case config.RepoLevel:
					*out.(*RepoConfig) = test.repo
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
			name:       "policy disabled",
			enabled:    false,
			repository: repositoryWithStatus("enabled"),
			expected: policydef.Result{
				Enabled: false,
				Pass:    true,
				Details: details{Status: "enabled", Available: true},
			},
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
				NotifyText: "Secret scanning not enabled.\nGitHub secret scanning looks for supported secret formats committed to a repository and creates alerts when it finds them.",
				Details:    details{Status: "disabled", Available: true},
			},
		},
		{
			name:       "status unavailable",
			enabled:    true,
			repository: &github.Repository{},
			expected: policydef.Result{
				Enabled: true,
				Pass:    true,
				Details: details{Status: "unavailable", Available: false},
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
			comparer := cmp.Comparer(func(left, right string) bool {
				return truncate(left, 140) == truncate(right, 140)
			})
			if diff := cmp.Diff(&test.expected, result, comparer); diff != "" {
				t.Fatalf("check() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFixEnablesSecretScanning(t *testing.T) {
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

func truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length]
}
