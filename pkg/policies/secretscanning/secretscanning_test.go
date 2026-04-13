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

	"github.com/google/go-github/v84/github"

	"github.com/ossf/allstar/pkg/config"
)

func TestCheck(t *testing.T) {
	configFetchConfig = func(ctx context.Context, c *github.Client, owner, repo, path string, ol config.ConfigLevel, out interface{}) error {
		return nil
	}
	configIsEnabled = func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig, c *github.Client, owner, repo string) (bool, error) {
		return true, nil
	}

	tests := []struct {
		name     string
		repo     *github.Repository
		wantPass bool
	}{
		{
			name: "secret scanning enabled",
			repo: &github.Repository{
				SecurityAndAnalysis: &github.SecurityAndAnalysis{
					SecretScanning: &github.SecretScanning{
						Status: github.Ptr("enabled"),
					},
				},
			},
			wantPass: true,
		},
		{
			name: "secret scanning disabled",
			repo: &github.Repository{
				SecurityAndAnalysis: &github.SecurityAndAnalysis{
					SecretScanning: &github.SecretScanning{
						Status: github.Ptr("disabled"),
					},
				},
			},
			wantPass: false,
		},
		{
			name:     "security and analysis nil",
			repo:     &github.Repository{},
			wantPass: false,
		},
		{
			name: "secret scanning nil",
			repo: &github.Repository{
				SecurityAndAnalysis: &github.SecurityAndAnalysis{},
			},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getRepo = func(ctx context.Context, c *github.Client, owner, repo string) (*github.Repository, *github.Response, error) {
				return tt.repo, nil, nil
			}

			res, err := check(context.Background(), nil, "testorg", "testrepo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Pass != tt.wantPass {
				t.Errorf("got pass=%v, want pass=%v", res.Pass, tt.wantPass)
			}
		})
	}
}
