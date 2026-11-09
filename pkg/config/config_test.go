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
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v84/github"
	"sigs.k8s.io/yaml"

	"github.com/ossf/allstar/pkg/config/operator"
)

type mockRepos struct {
	getContents func(context.Context, string, string, string,
		*github.RepositoryContentGetOptions) (*github.RepositoryContent,
		[]*github.RepositoryContent, *github.Response, error)

	get func(context.Context, string, string) (*github.Repository,
		*github.Response, error)
}

func (m mockRepos) GetContents(ctx context.Context, owner, repo, path string,
	opts *github.RepositoryContentGetOptions) (*github.RepositoryContent,
	[]*github.RepositoryContent, *github.Response, error,
) {
	return m.getContents(ctx, owner, repo, path, opts)
}

func (m mockRepos) Get(ctx context.Context, owner, repo string) (*github.Repository,
	*github.Response, error,
) {
	return m.get(ctx, owner, repo)
}

//nolint:paralleltest // Replaces walkGC and shares the installation cache.
func TestFetchConfig(t *testing.T) {
	oldWalkGC := walkGC
	ClearInstLoc("")
	t.Cleanup(func() { walkGC = oldWalkGC; ClearInstLoc("") })
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
  optOutPrivateRepos: true
  optOutPublicRepos: true
  disableRepoOverride: true
`,
			Expect: &OrgConfig{
				OptConfig: OrgOptConfig{
					OptOutStrategy:      true,
					OptOutRepos:         []string{"repo1", "repo2"},
					OptOutPrivateRepos:  true,
					OptOutPublicRepos:   true,
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
			Name: "IssueLabel",
			Input: `
issueLabel: testlabel
`,
			Expect: &OrgConfig{
				IssueLabel: "testlabel",
			},
			Got: &OrgConfig{},
		},
		{
			Name: "IssueRepo",
			Input: `
issueRepo: testrepository
`,
			Expect: &OrgConfig{
				IssueRepo: "testrepository",
			},
			Got: &OrgConfig{},
		},
		{
			Name: "IssueFooter",
			Input: `
issueFooter: testfooter
`,
			Expect: &OrgConfig{
				IssueFooter: "testfooter",
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
		{
			Name: "IssueDetails",
			Input: `
issueDetails: test Issue Detail
`,
			Expect: &OrgConfig{
				IssueDetails: "test Issue Detail",
			},
			Got: &OrgConfig{},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			walkGC = func(ctx context.Context, r repositories, owner, repo, path string,
				opts *github.RepositoryContentGetOptions) (*github.RepositoryContent,
				[]*github.RepositoryContent, *github.Response, error,
			) {
				e := "base64"
				c := base64.StdEncoding.EncodeToString([]byte(test.Input))
				return &github.RepositoryContent{
					Encoding: &e,
					Content:  &c,
				}, nil, nil, nil
			}
			get := func(ctx context.Context, owner, repo string) (*github.Repository,
				*github.Response, error,
			) {
				return nil, nil, nil
			}
			err := fetchConfig(context.Background(), mockRepos{get: get}, "", "", "", OrgLevel, test.Got)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff(test.Expect, test.Got); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}

//nolint:paralleltest // Writes the shared glob cache.
func TestIsEnabled(t *testing.T) {
	tests := []struct {
		Name           string
		Org            OrgOptConfig
		OrgRepo        RepoOptConfig
		Repo           RepoOptConfig
		IsPrivateRepo  bool
		IsArchivedRepo bool
		IsForkedRepo   bool
		Expect         bool
	}{
		{
			Name: "OptInOrg",
			Org: OrgOptConfig{
				OptOutStrategy: false,
				OptInRepos:     []string{"thisrepo"},
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        true,
		},
		{
			Name: "OptInOrg",
			Org: OrgOptConfig{
				OptOutStrategy: false,
				OptInRepos:     []string{"this*"},
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        true,
		},
		{
			Name: "NoOptInOrg",
			Org: OrgOptConfig{
				OptOutStrategy: false,
				OptInRepos:     []string{"otherrepo"},
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        false,
		},
		{
			Name: "NoOptInOrg",
			Org: OrgOptConfig{
				OptOutStrategy: false,
				OptInRepos:     []string{"other*"},
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        false,
		},
		{
			Name: "NoOptInOrg",
			Org: OrgOptConfig{
				OptOutStrategy: false,
				OptInRepos:     []string{"this*xyz"},
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        false,
		},
		{
			Name: "OptOutOrg",
			Org: OrgOptConfig{
				OptOutStrategy: true,
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        true,
		},
		{
			Name: "NoOptOutOrg",
			Org: OrgOptConfig{
				OptOutStrategy: true,
				OptOutRepos:    []string{"thisrepo"},
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        false,
		},
		{
			Name: "OptOutPrivateRepos",
			Org: OrgOptConfig{
				OptOutStrategy:     true,
				OptOutPrivateRepos: true,
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: true,
			Expect:        false,
		},
		{
			Name: "NoOptOutPrivateRepos",
			Org: OrgOptConfig{
				OptOutStrategy:     true,
				OptOutPrivateRepos: false,
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: true,
			Expect:        true,
		},
		{
			Name: "OptOutPublicRepos",
			Org: OrgOptConfig{
				OptOutStrategy:    true,
				OptOutPublicRepos: true,
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        false,
		},
		{
			Name: "NoOptOutPublicRepos",
			Org: OrgOptConfig{
				OptOutStrategy:    true,
				OptOutPublicRepos: false,
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        true,
		},
		{
			Name: "OptOutArchivedRepos",
			Org: OrgOptConfig{
				OptOutStrategy:      true,
				OptOutArchivedRepos: true,
			},
			OrgRepo:        RepoOptConfig{},
			Repo:           RepoOptConfig{},
			IsPrivateRepo:  true,
			IsArchivedRepo: true,
			Expect:         false,
		},
		{
			Name: "NoOptOutArchivedRepos",
			Org: OrgOptConfig{
				OptOutStrategy: true,
			},
			OrgRepo:        RepoOptConfig{},
			Repo:           RepoOptConfig{},
			IsPrivateRepo:  true,
			IsArchivedRepo: true,
			Expect:         true,
		},
		{
			Name: "OptOutForkedRepos",
			Org: OrgOptConfig{
				OptOutStrategy:    true,
				OptOutForkedRepos: true,
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: true,
			IsForkedRepo:  true,
			Expect:        false,
		},
		{
			Name: "NoOptOutForkedRepos",
			Org: OrgOptConfig{
				OptOutStrategy: true,
			},
			OrgRepo:       RepoOptConfig{},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: true,
			IsForkedRepo:  true,
			Expect:        true,
		},
		{
			Name:    "RepoOptIn",
			Org:     OrgOptConfig{},
			OrgRepo: RepoOptConfig{},
			Repo: RepoOptConfig{
				OptIn: true,
			},
			IsPrivateRepo: false,
			Expect:        true,
		},
		{
			Name: "RepoOptOut",
			Org: OrgOptConfig{
				OptOutStrategy: true,
			},
			OrgRepo: RepoOptConfig{},
			Repo: RepoOptConfig{
				OptOut: true,
			},
			IsPrivateRepo: false,
			Expect:        false,
		},
		{
			Name: "DisallowOptOut",
			Org: OrgOptConfig{
				OptOutStrategy:      true,
				DisableRepoOverride: true,
			},
			OrgRepo: RepoOptConfig{},
			Repo: RepoOptConfig{
				OptOut: true,
			},
			IsPrivateRepo: false,
			Expect:        true,
		},
		{
			Name: "OrgRepoOptIn",
			Org:  OrgOptConfig{},
			OrgRepo: RepoOptConfig{
				OptIn: true,
			},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        true,
		},
		{
			Name: "OrgRepoOptOut",
			Org: OrgOptConfig{
				OptOutStrategy: true,
			},
			OrgRepo: RepoOptConfig{
				OptOut: true,
			},
			Repo:          RepoOptConfig{},
			IsPrivateRepo: false,
			Expect:        false,
		},
		{
			Name: "OrgRepoOptOutRepoOptIn",
			Org: OrgOptConfig{
				OptOutStrategy: true,
			},
			OrgRepo: RepoOptConfig{
				OptOut: false,
			},
			Repo: RepoOptConfig{
				OptOut: true,
			},
			IsPrivateRepo: false,
			Expect:        false,
		},
		{
			Name: "DisallowWithOrgRepo",
			Org: OrgOptConfig{
				OptOutStrategy:      true,
				DisableRepoOverride: true,
			},
			OrgRepo: RepoOptConfig{
				OptOut: true,
			},
			Repo: RepoOptConfig{
				OptOut: true,
			},
			IsPrivateRepo: false,
			Expect:        false,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			get := func(context.Context, string, string) (*github.Repository,
				*github.Response, error,
			) {
				return &github.Repository{
					Private:  &test.IsPrivateRepo,
					Archived: &test.IsArchivedRepo,
					Fork:     &test.IsForkedRepo,
				}, nil, nil
			}
			got, err := isEnabled(context.Background(), test.Org, test.OrgRepo, test.Repo, mockRepos{get: get}, "thisorg", "thisrepo")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if got != test.Expect {
				t.Errorf("Unexpected results on %v. Expected: %v", test.Name, test.Expect)
			}
		})
	}
}

//nolint:paralleltest // Replaces walkGC and shares the installation cache.
func TestIsBotEnabled(t *testing.T) {
	oldWalkGC := walkGC
	ClearInstLoc("")
	t.Cleanup(func() { walkGC = oldWalkGC; ClearInstLoc("") })
	get := func(context.Context, string, string) (*github.Repository, *github.Response, error) {
		return &github.Repository{}, nil, nil
	}
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
	walkGC = func(ctx context.Context, r repositories, owner, repo, path string,
		opts *github.RepositoryContentGetOptions) (*github.RepositoryContent,
		[]*github.RepositoryContent, *github.Response, error,
	) {
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

	if !isBotEnabled(context.Background(), mockRepos{get: get}, "", "thisrepo") {
		t.Error("Expected repo to be enabled")
	}
}

func TestCreateIL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name       string
		DotAllstar bool
		DotGitHub  bool
		Expect     *instLoc
	}{
		{
			Name:       "Allstar exists",
			DotAllstar: true,
			DotGitHub:  true,
			Expect: &instLoc{
				Exists: true,
				Repo:   operator.OrgConfigRepo,
				Path:   "",
			},
		},
		{
			Name:       "Allstar exists2",
			DotAllstar: true,
			DotGitHub:  false,
			Expect: &instLoc{
				Exists: true,
				Repo:   operator.OrgConfigRepo,
				Path:   "",
			},
		},
		{
			Name:       "Dot GitHub",
			DotAllstar: false,
			DotGitHub:  true,
			Expect: &instLoc{
				Exists: true,
				Repo:   githubConfRepo,
				Path:   operator.OrgConfigDir,
			},
		},
		{
			Name:       "Neither",
			DotAllstar: false,
			DotGitHub:  false,
			Expect: &instLoc{
				Exists: false,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			get := func(ctx context.Context, owner, repo string) (*github.Repository,
				*github.Response, error,
			) {
				if repo == operator.OrgConfigRepo && test.DotAllstar {
					return nil, nil, nil
				}
				if repo == githubConfRepo && test.DotGitHub {
					return nil, nil, nil
				}
				return nil, &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("Not found")
			}
			got, err := createIl(context.Background(), mockRepos{get: get}, "")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff(test.Expect, got); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetIL(t *testing.T) {
	t.Parallel()

	owner, otherOwner := t.Name(), t.Name()+"-other"
	t.Cleanup(func() { ClearInstLoc(owner); ClearInstLoc(otherOwner) })
	var getCalled bool
	get := func(ctx context.Context, owner, repo string) (*github.Repository,
		*github.Response, error,
	) {
		getCalled = true
		return nil, nil, nil
	}
	getCalled = false
	if _, err := getInstLoc(context.Background(), mockRepos{get: get}, owner); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !getCalled {
		t.Errorf("Get not called")
	}
	getCalled = false
	if _, err := getInstLoc(context.Background(), mockRepos{get: get}, owner); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if getCalled {
		t.Errorf("Get called on second lookup")
	}
	ClearInstLoc(owner)
	getCalled = false
	if _, err := getInstLoc(context.Background(), mockRepos{get: get}, owner); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !getCalled {
		t.Errorf("Get not called after clear")
	}
	getCalled = false
	if _, err := getInstLoc(context.Background(), mockRepos{get: get}, otherOwner); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !getCalled {
		t.Errorf("Get not called on second org")
	}
}

func TestWalkGetContents(t *testing.T) {
	t.Parallel()

	p := "long/path/file.yaml"
	expect := []string{"", "long/", "long/path/", "long/path/file.yaml"}
	var got []string
	getContents := func(ctx context.Context, owner, repo, pt string,
		opts *github.RepositoryContentGetOptions) (*github.RepositoryContent,
		[]*github.RepositoryContent, *github.Response, error,
	) {
		got = append(got, pt)
		if strings.HasSuffix(pt, ".yaml") {
			e := "base64"
			c := base64.StdEncoding.EncodeToString([]byte("asdf"))
			return &github.RepositoryContent{
				Encoding: &e,
				Content:  &c,
			}, nil, nil, nil
		} else {
			return nil, []*github.RepositoryContent{ // All three are always there
				{Name: github.Ptr("long")},
				{Name: github.Ptr("path")},
				{Name: github.Ptr("file.yaml")},
			}, nil, nil
		}
	}
	content, _, _, err := walkGetContents(context.Background(), mockRepos{getContents: getContents}, "", "", p, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if content == nil {
		t.Fatal("Expected file content")
	}
	if diff := cmp.Diff(expect, got); diff != "" {
		t.Errorf("Unexpected results. (-want +got):\n%s", diff)
	}

	expect2 := []string{"", "long/"}
	var got2 []string
	getContents = func(ctx context.Context, owner, repo, pt string,
		opts *github.RepositoryContentGetOptions) (*github.RepositoryContent,
		[]*github.RepositoryContent, *github.Response, error,
	) {
		got2 = append(got2, pt)
		if strings.HasSuffix(pt, ".yaml") {
			e := "base64"
			c := base64.StdEncoding.EncodeToString([]byte("asdf"))
			return &github.RepositoryContent{
				Encoding: &e,
				Content:  &c,
			}, nil, nil, nil
		} else {
			return nil, []*github.RepositoryContent{ // path is not there
				{Name: github.Ptr("long")},
				{Name: github.Ptr("file.yaml")},
			}, nil, nil
		}
	}
	_, _, rsp, err := walkGetContents(context.Background(), mockRepos{getContents: getContents}, "", "", p, nil)
	if !errors.Is(err, errContentNotFound) {
		t.Errorf("Expected content-not-found error, got: %v", err)
	}
	if diff := cmp.Diff(expect2, got2); diff != "" {
		t.Errorf("Unexpected results. (-want +got):\n%s", diff)
	}
	if rsp == nil || rsp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status not found, got: %v", rsp)
	}
}

func TestMerge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name   string
		Input  string
		Base   string
		Expect string
	}{
		{
			Name: "NoMerge",
			Input: `
foo: asdf
barBaz: qwer
`,
			Base: "",
			Expect: `barBaz: qwer
foo: asdf
`,
		},
		{
			Name: "MergeNoChange",
			Input: `
baseConfig: poiu/lkjh
`,
			Base: `
foo: asdf
barBaz: qwer
`,
			Expect: `barBaz: qwer
baseConfig: poiu/lkjh
foo: asdf
`,
		},
		{
			Name: "MergeNoBase",
			Input: `
baseConfig: poiu/lkjh
foo: asdf
`,
			Base: "",
			Expect: `baseConfig: poiu/lkjh
foo: asdf
`,
		},
		{
			Name: "MergeAdd",
			Input: `
baseConfig: poiu/lkjh
foo: asdf
`,
			Base: `
barBaz: qwer
`,
			Expect: `barBaz: qwer
baseConfig: poiu/lkjh
foo: asdf
`,
		},
		{
			Name: "MergeOverride",
			Input: `
baseConfig: poiu/lkjh
foo: asdf
`,
			Base: `
foo: foo
barBaz: qwer
`,
			Expect: `barBaz: qwer
baseConfig: poiu/lkjh
foo: asdf
`,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			getContents := func(ctx context.Context, owner, repo, path string,
				opts *github.RepositoryContentGetOptions) (*github.RepositoryContent,
				[]*github.RepositoryContent, *github.Response, error,
			) {
				// check owner / repo / path??
				e := "base64"
				c := base64.StdEncoding.EncodeToString([]byte(test.Base))
				return &github.RepositoryContent{
					Encoding: &e,
					Content:  &c,
				}, nil, nil, nil
			}
			conJSON, err := yaml.YAMLToJSON([]byte(test.Input))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			got, err := checkAndMergeBase(context.Background(), mockRepos{getContents: getContents}, "path", conJSON)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			gotYAML, err := yaml.JSONToYAML(got)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff(test.Expect, string(gotYAML)); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWalkGetContentsPreservesResponseAndError(t *testing.T) {
	t.Parallel()
	for _, failDirectory := range []bool{true, false} {
		name := "file"
		if failDirectory {
			name = "directory"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rsp := &github.Response{Response: &http.Response{StatusCode: http.StatusInternalServerError}}
			apiErr := &github.ErrorResponse{Response: rsp.Response, Message: "unavailable"}
			partial := &github.RepositoryContent{Name: github.Ptr("file.yaml")}
			r := mockRepos{getContents: func(_ context.Context, _, _, p string,
				_ *github.RepositoryContentGetOptions,
			) (*github.RepositoryContent, []*github.RepositoryContent, *github.Response, error) {
				if p == "" && !failDirectory {
					return nil, []*github.RepositoryContent{partial}, nil, nil
				}
				return partial, nil, rsp, apiErr
			}}
			content, _, gotRsp, err := walkGetContents(t.Context(), r, "owner", "repo", "file.yaml", nil)
			if !errors.Is(err, apiErr) || errors.Unwrap(err) == nil {
				t.Fatalf("error = %v, want wrapped API error", err)
			}
			var gotAPIError *github.ErrorResponse
			if !errors.As(err, &gotAPIError) || gotAPIError != apiErr {
				t.Errorf("error does not preserve the GitHub error type: %v", err)
			}
			if gotRsp != rsp {
				t.Error("GitHub response was not preserved")
			}
			if !failDirectory && content != partial {
				t.Error("partial file content was not preserved")
			}
		})
	}
}

func TestWalkGetContentsEmptyDirectory(t *testing.T) {
	t.Parallel()
	rsp := &github.Response{Response: &http.Response{StatusCode: http.StatusOK}}
	r := mockRepos{getContents: func(context.Context, string, string, string,
		*github.RepositoryContentGetOptions,
	) (*github.RepositoryContent, []*github.RepositoryContent, *github.Response, error) {
		return nil, nil, rsp, nil
	}}
	_, _, gotRsp, err := walkGetContents(t.Context(), r, "owner", "repo", "file.yaml", nil)
	if err != nil || gotRsp != rsp {
		t.Errorf("empty directory response = %v, error = %v; want original response and nil error", gotRsp, err)
	}
}
