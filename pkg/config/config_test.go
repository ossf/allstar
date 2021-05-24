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

package config

import (
	"context"
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/google/go-github/v35/github"
)

var getContents func(context.Context, string, string, string,
	*github.RepositoryContentGetOptions) (*github.RepositoryContent,
	[]*github.RepositoryContent, *github.Response, error)

type mockRepos struct{}

func (m mockRepos) GetContents(ctx context.Context, owner, repo, path string,
	opts *github.RepositoryContentGetOptions) (*github.RepositoryContent,
	[]*github.RepositoryContent, *github.Response, error) {
	return getContents(ctx, owner, repo, path, opts)
}

func TestFetchConfig(t *testing.T) {
	tests := []struct {
		Name   string
		Input  string
		Expect interface{}
		Got    interface{}
	}{
		{
			Name: "OptOutOrg",
			Input: `
optConfig:
  optOutStrategy: true
  optOutRepos:
  - repo1
  - repo2
  disableRepoOverride: true
`,
			Expect: &OrgConfig{
				OptConfig: OrgOptConfig{
					OptOutStrategy:      true,
					OptOutRepos:         []string{"repo1", "repo2"},
					DisableRepoOverride: true,
				},
			},
			Got: &OrgConfig{},
		},
		{
			Name: "OptInOrg",
			Input: `
optConfig:
  optOutStrategy: false
  optInRepos:
  - repo1
  - repo2
`,
			Expect: &OrgConfig{
				OptConfig: OrgOptConfig{
					OptOutStrategy:      false,
					OptInRepos:          []string{"repo1", "repo2"},
					DisableRepoOverride: false,
				},
			},
			Got: &OrgConfig{},
		},
		{
			Name: "OptOutRepo",
			Input: `
optConfig:
  optOut: true
`,
			Expect: &RepoConfig{
				OptConfig: RepoOptConfig{
					OptIn:  false,
					OptOut: true,
				},
			},
			Got: &RepoConfig{},
		},
		{
			Name: "OptInRepo",
			Input: `
optConfig:
  optIn: true
`,
			Expect: &RepoConfig{
				OptConfig: RepoOptConfig{
					OptIn:  true,
					OptOut: false,
				},
			},
			Got: &RepoConfig{},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			getContents = func(ctx context.Context, owner, repo, path string,
				opts *github.RepositoryContentGetOptions) (*github.RepositoryContent,
				[]*github.RepositoryContent, *github.Response, error) {
				e := "base64"
				c := base64.StdEncoding.EncodeToString([]byte(test.Input))
				return &github.RepositoryContent{
					Encoding: &e,
					Content:  &c,
				}, nil, nil, nil
			}
			err := fetchConfig(context.Background(), mockRepos{}, "", "", "", test.Got)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !reflect.DeepEqual(test.Got, test.Expect) {
				t.Errorf("Unexpected results. Got: %v, Expect: %v", test.Got, test.Expect)
			}
		})
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		Name   string
		Org    OrgOptConfig
		Repo   RepoOptConfig
		Expect bool
	}{
		{
			Name: "OptInOrg",
			Org: OrgOptConfig{
				OptOutStrategy: false,
				OptInRepos:     []string{"thisrepo"},
			},
			Repo:   RepoOptConfig{},
			Expect: true,
		},
		{
			Name: "NoOptInOrg",
			Org: OrgOptConfig{
				OptOutStrategy: false,
				OptInRepos:     []string{"otherrepo"},
			},
			Repo:   RepoOptConfig{},
			Expect: false,
		},
		{
			Name: "OptOutOrg",
			Org: OrgOptConfig{
				OptOutStrategy: true,
			},
			Repo:   RepoOptConfig{},
			Expect: true,
		},
		{
			Name: "NoOptOutOrg",
			Org: OrgOptConfig{
				OptOutStrategy: true,
				OptOutRepos:    []string{"thisrepo"},
			},
			Repo:   RepoOptConfig{},
			Expect: false,
		},
		{
			Name: "RepoOptIn",
			Org:  OrgOptConfig{},
			Repo: RepoOptConfig{
				OptIn: true,
			},
			Expect: true,
		},
		{
			Name: "RepoOptOut",
			Org: OrgOptConfig{
				OptOutStrategy: true,
			},
			Repo: RepoOptConfig{
				OptOut: true,
			},
			Expect: false,
		},
		{
			Name: "DissallowOptOut",
			Org: OrgOptConfig{
				OptOutStrategy:      true,
				DisableRepoOverride: true,
			},
			Repo: RepoOptConfig{
				OptOut: true,
			},
			Expect: true,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if IsEnabled(test.Org, test.Repo, "thisrepo") != test.Expect {
				t.Errorf("Unexpected results. Expected: %v", test.Expect)
			}
		})
	}
}

func TestIsBotEnabled(t *testing.T) {
	// FetchConfig and IsEnabled are both tested, just do one test case here
	orgIn := `
optConfig:
  optOutStrategy: false
  optInRepos:
  - thisrepo
  - repo2
  disableRepoOverride: true
`
	repoIn := `
optConfig:
  optOut: true
`
	getContents = func(ctx context.Context, owner, repo, path string,
		opts *github.RepositoryContentGetOptions) (*github.RepositoryContent,
		[]*github.RepositoryContent, *github.Response, error) {
		e := "base64"
		var c string
		if repo == "thisrepo" {
			c = base64.StdEncoding.EncodeToString([]byte(repoIn))
		} else {
			c = base64.StdEncoding.EncodeToString([]byte(orgIn))
		}
		return &github.RepositoryContent{
			Encoding: &e,
			Content:  &c,
		}, nil, nil, nil
	}

	if !isBotEnabled(context.Background(), mockRepos{}, "", "thisrepo") {
		t.Error("Expected repo to be enabled")
	}
}
