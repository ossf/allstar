// Copyright 2026 Allstar Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package branch

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-github/v84/github"

	"github.com/ossf/allstar/pkg/config"
)

// notFound is what GitHub answers on the classic branch protection endpoints when the
// setting is not there to be written, which is what a repository migrated to rulesets
// looks like from the outside.
func notFound() (*github.Response, error) {
	return &github.Response{
		Response: &http.Response{StatusCode: http.StatusNotFound},
	}, errors.New("404 Not Found")
}

// A 404 from UpdateBranchProtection must not end the run. Before this was handled, the
// error travelled up out of Fix and stopped enforcement for every repository queued
// behind this one, so the branch that follows the failing one is what the test watches:
// checking only that Fix returned no error would also pass if the loop had quietly
// stopped after the first branch.
func TestFixContinuesWhenBranchProtectionIsNotAvailable(t *testing.T) {
	attempted := make([]string, 0)

	get = func(context.Context, string, string) (*github.Repository, *github.Response, error) {
		return &github.Repository{DefaultBranch: github.Ptr("main")}, nil, nil
	}
	configFetchConfig = func(ctx context.Context, c *github.Client,
		owner, repo, path string, ol config.ConfigLevel, out interface{},
	) error {
		if ol == config.OrgLevel {
			oc := out.(*OrgConfig)
			*oc = OrgConfig{
				EnforceDefault:  true,
				EnforceBranches: map[string][]string{"thisrepo": {"migrated"}},
				RequireApproval: true,
				ApprovalCount:   1,
			}
		}
		return nil
	}
	configIsEnabled = func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig,
		c *github.Client, owner, repo string,
	) (bool, error) {
		return true, nil
	}
	// No classic protection anywhere, so every branch takes the "create from config" path.
	getBranchProtection = func(ctx context.Context, o, r, b string) (*github.Protection, *github.Response, error) {
		rsp, err := notFound()
		return nil, rsp, err
	}
	updateBranchProtection = func(ctx context.Context, owner, repo, branch string,
		preq *github.ProtectionRequest,
	) (*github.Protection, *github.Response, error) {
		attempted = append(attempted, branch)
		if branch == "migrated" {
			rsp, err := notFound()
			return nil, rsp, err
		}
		return nil, nil, nil
	}
	getSignaturesProtectedBranch = func(ctx context.Context, o, r, b string) (
		*github.SignaturesProtectedBranch, *github.Response, error,
	) {
		return &github.SignaturesProtectedBranch{Enabled: github.Ptr(false)}, nil, nil
	}
	requireSignaturesProtectedBranch = func(ctx context.Context, owner, repo, branch string) (
		*github.SignaturesProtectedBranch, *github.Response, error,
	) {
		return nil, nil, nil
	}

	if err := fix(context.Background(), mockRepos{}, nil, "", "thisrepo"); err != nil {
		t.Fatalf("a branch without available protection must not fail the run: %v", err)
	}
	if !contains(attempted, "main") {
		t.Errorf("the run stopped at the first branch: protection was attempted for %v, want the default branch too", attempted)
	}
}

// The same applies to the signed commits call: it is made per branch on protection that
// is not there either, so its 404 used to end the run just as effectively.
func TestFixContinuesWhenSignedCommitsCannotBeRequired(t *testing.T) {
	attempted := make([]string, 0)
	protection := &github.Protection{
		AllowForcePushes: &github.AllowForcePushes{Enabled: false},
		EnforceAdmins:    &github.AdminEnforcement{Enabled: false},
	}

	get = func(context.Context, string, string) (*github.Repository, *github.Response, error) {
		return &github.Repository{DefaultBranch: github.Ptr("main")}, nil, nil
	}
	configFetchConfig = func(ctx context.Context, c *github.Client,
		owner, repo, path string, ol config.ConfigLevel, out interface{},
	) error {
		if ol == config.OrgLevel {
			oc := out.(*OrgConfig)
			*oc = OrgConfig{
				EnforceDefault:       true,
				EnforceBranches:      map[string][]string{"thisrepo": {"migrated"}},
				RequireSignedCommits: true,
			}
		}
		return nil
	}
	configIsEnabled = func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig,
		c *github.Client, owner, repo string,
	) (bool, error) {
		return true, nil
	}
	getBranchProtection = func(ctx context.Context, o, r, b string) (*github.Protection, *github.Response, error) {
		return protection, nil, nil
	}
	updateBranchProtection = func(ctx context.Context, owner, repo, branch string,
		preq *github.ProtectionRequest,
	) (*github.Protection, *github.Response, error) {
		return nil, nil, nil
	}
	getSignaturesProtectedBranch = func(ctx context.Context, o, r, b string) (
		*github.SignaturesProtectedBranch, *github.Response, error,
	) {
		return &github.SignaturesProtectedBranch{Enabled: github.Ptr(false)}, nil, nil
	}
	requireSignaturesProtectedBranch = func(ctx context.Context, owner, repo, branch string) (
		*github.SignaturesProtectedBranch, *github.Response, error,
	) {
		attempted = append(attempted, branch)
		if branch == "migrated" {
			rsp, err := notFound()
			return nil, rsp, err
		}
		return nil, nil, nil
	}

	if err := fix(context.Background(), mockRepos{}, nil, "", "thisrepo"); err != nil {
		t.Fatalf("a branch that cannot take signed commits must not fail the run: %v", err)
	}
	if !contains(attempted, "main") {
		t.Errorf("the run stopped at the first branch: signed commits were attempted for %v, want the default branch too", attempted)
	}
}

// A 404 is the only status treated this way: anything else is still a real failure and
// has to reach the caller, otherwise a broken installation would look healthy.
func TestFixStillFailsOnOtherErrors(t *testing.T) {
	get = func(context.Context, string, string) (*github.Repository, *github.Response, error) {
		return &github.Repository{DefaultBranch: github.Ptr("main")}, nil, nil
	}
	configFetchConfig = func(ctx context.Context, c *github.Client,
		owner, repo, path string, ol config.ConfigLevel, out interface{},
	) error {
		if ol == config.OrgLevel {
			oc := out.(*OrgConfig)
			*oc = OrgConfig{EnforceDefault: true, RequireApproval: true, ApprovalCount: 1}
		}
		return nil
	}
	configIsEnabled = func(ctx context.Context, o config.OrgOptConfig, orc, r config.RepoOptConfig,
		c *github.Client, owner, repo string,
	) (bool, error) {
		return true, nil
	}
	getBranchProtection = func(ctx context.Context, o, r, b string) (*github.Protection, *github.Response, error) {
		rsp, err := notFound()
		return nil, rsp, err
	}
	updateBranchProtection = func(ctx context.Context, owner, repo, branch string,
		preq *github.ProtectionRequest,
	) (*github.Protection, *github.Response, error) {
		return nil, &github.Response{
			Response: &http.Response{StatusCode: http.StatusInternalServerError},
		}, errors.New("500 Internal Server Error")
	}
	getSignaturesProtectedBranch = func(ctx context.Context, o, r, b string) (
		*github.SignaturesProtectedBranch, *github.Response, error,
	) {
		return &github.SignaturesProtectedBranch{Enabled: github.Ptr(false)}, nil, nil
	}
	requireSignaturesProtectedBranch = func(ctx context.Context, owner, repo, branch string) (
		*github.SignaturesProtectedBranch, *github.Response, error,
	) {
		return nil, nil, nil
	}

	if err := fix(context.Background(), mockRepos{}, nil, "", "thisrepo"); err == nil {
		t.Error("a server error must still be reported, only a missing setting is skipped")
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
