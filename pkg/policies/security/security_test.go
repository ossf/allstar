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
	"encoding/base64"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/go-github/v35/github"
	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
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

func TestCheck(t *testing.T) {
	tests := []struct {
		Name     string
		Org      OrgConfig
		Repo     RepoConfig
		Exists   bool
		Contents string
		Exp      policydef.Result
	}{
		{
			Name:     "NotEnabled",
			Org:      OrgConfig{},
			Repo:     RepoConfig{},
			Exists:   true,
			Contents: "notempty",
			Exp: policydef.Result{
				Enabled:    false,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Exists: true,
					Empty:  false,
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
			Repo:     RepoConfig{},
			Exists:   true,
			Contents: "notempty",
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       true,
				NotifyText: "",
				Details: details{
					Exists: true,
					Empty:  false,
				},
			},
		},
		{
			Name: "Missing",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
			},
			Repo:     RepoConfig{},
			Exists:   false,
			Contents: "",
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "SECURITY.md not found.\nGo to https://github.com//thisrepo/security/policy to enable.\n",
				Details: details{
					Exists: false,
					Empty:  true,
				},
			},
		},
		{
			Name: "Empty",
			Org: OrgConfig{
				OptConfig: config.OrgOptConfig{
					OptOutStrategy: true,
				},
			},
			Repo:     RepoConfig{},
			Exists:   true,
			Contents: "",
			Exp: policydef.Result{
				Enabled:    true,
				Pass:       false,
				NotifyText: "SECURITY.md is empty.\n",
				Details: details{
					Exists: true,
					Empty:  true,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configFetchConfig = func(ctx context.Context, c *github.Client,
				owner string, repo string, path string, out interface{}) error {
				if repo == "thisrepo" {
					rc := out.(*RepoConfig)
					*rc = test.Repo
				} else {
					oc := out.(*OrgConfig)
					*oc = test.Org
				}
				return nil
			}
			getContents = func(ctx context.Context, owner, repo, path string,
				opts *github.RepositoryContentGetOptions) (*github.RepositoryContent,
				[]*github.RepositoryContent, *github.Response, error) {
				if test.Exists {
					e := "base64"
					c := base64.StdEncoding.EncodeToString([]byte(test.Contents))
					s := len(test.Contents)
					return &github.RepositoryContent{
						Encoding: &e,
						Content:  &c,
						Size:     &s,
					}, nil, nil, nil
				} else {
					return nil, nil, &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("Not Found")
				}
			}
			res, err := check(context.Background(), mockRepos{}, nil, "", "thisrepo")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !reflect.DeepEqual(res, &test.Exp) {
				t.Errorf("Unexpected results. Got: %v, Expect: %v", res, &test.Exp)
			}
		})
	}
}
