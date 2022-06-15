// Copyright 2022 Allstar Authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scorecard

import (
	"context"
	"net/http"
	"testing"

	"github.com/ossf/scorecard/v4/clients"
)

var initRepo func(clients.Repo, string) error
var close func() error

type mockRC struct{}

func (m mockRC) InitRepo(r clients.Repo, s string) error {
	return initRepo(r, s)
}

func (m mockRC) URI() string {
	return ""
}

func (m mockRC) IsArchived() (bool, error) {
	return false, nil
}

func (m mockRC) ListFiles(predicate func(string) (bool, error)) ([]string, error) {
	return nil, nil
}

func (m mockRC) GetFileContent(filename string) ([]byte, error) {
	return nil, nil
}

func (m mockRC) ListBranches() ([]*clients.BranchRef, error) {
	return nil, nil
}

func (m mockRC) GetBranch(branch string) (*clients.BranchRef, error) {
	return nil, nil
}

func (m mockRC) GetDefaultBranch() (*clients.BranchRef, error) {
	return nil, nil
}
func (m mockRC) ListCommits() ([]clients.Commit, error) {
	return nil, nil
}

func (m mockRC) ListIssues() ([]clients.Issue, error) {
	return nil, nil
}

func (m mockRC) ListReleases() ([]clients.Release, error) {
	return nil, nil
}

func (m mockRC) ListContributors() ([]clients.User, error) {
	return nil, nil
}

func (m mockRC) ListSuccessfulWorkflowRuns(filename string) ([]clients.WorkflowRun, error) {
	return nil, nil
}

func (m mockRC) ListCheckRunsForRef(ref string) ([]clients.CheckRun, error) {
	return nil, nil
}

func (m mockRC) ListStatuses(ref string) ([]clients.Status, error) {
	return nil, nil
}

func (m mockRC) ListWebhooks() ([]clients.Webhook, error) {
	return nil, nil
}

func (m mockRC) ListProgrammingLanguages() ([]clients.Language, error) {
	return nil, nil
}

func (m mockRC) Search(request clients.SearchRequest) (clients.SearchResponse, error) {
	return clients.SearchResponse{}, nil
}

func (m mockRC) Close() error {
	return close()
}

func TestGetNew(t *testing.T) {
	var makeCalled, createCalled, initCalled bool
	githubrepoMakeGithubRepo = func(s string) (clients.Repo, error) {
		makeCalled = true
		return nil, nil
	}
	githubrepoCreateGithubRepoClientWithTransport = func(c context.Context, tr http.RoundTripper) clients.RepoClient {
		createCalled = true
		return mockRC{}
	}
	initRepo = func(r clients.Repo, s string) error {
		initCalled = true
		return nil
	}
	close = func() error {
		return nil
	}
	_, _ = Get(context.Background(), "org/repo", nil)
	if !makeCalled {
		t.Error("githubrepo.MakeGithubRepo not called for new repo")
	}
	if !createCalled {
		t.Error("githubrepo.CreateGithubRepoClientWithTransport not called for new repo")
	}
	if !initCalled {
		t.Error("repoclient.InitRepo not called for new repo")
	}
	Close("org/repo")
}

func TestGetExisting(t *testing.T) {
	var makeCalled, createCalled, initCalled bool
	githubrepoMakeGithubRepo = func(s string) (clients.Repo, error) {
		makeCalled = true
		return nil, nil
	}
	githubrepoCreateGithubRepoClientWithTransport = func(c context.Context, tr http.RoundTripper) clients.RepoClient {
		createCalled = true
		return mockRC{}
	}
	initRepo = func(r clients.Repo, s string) error {
		initCalled = true
		return nil
	}
	close = func() error {
		return nil
	}
	_, _ = Get(context.Background(), "org/repo", nil)
	makeCalled, createCalled, initCalled = false, false, false
	_, _ = Get(context.Background(), "org/repo", nil)
	if makeCalled {
		t.Error("githubrepo.MakeGithubRepo called for existing repo")
	}
	if createCalled {
		t.Error("githubrepo.CreateGithubRepoClientWithTransport called for existing repo")
	}
	if initCalled {
		t.Error("repoclient.InitRepo called for existing repo")
	}
	Close("org/repo")
}

func TestClose(t *testing.T) {
	var closeCalled bool
	githubrepoMakeGithubRepo = func(s string) (clients.Repo, error) {
		return nil, nil
	}
	githubrepoCreateGithubRepoClientWithTransport = func(c context.Context, tr http.RoundTripper) clients.RepoClient {
		return mockRC{}
	}
	initRepo = func(r clients.Repo, s string) error {
		return nil
	}
	close = func() error {
		closeCalled = true
		return nil
	}
	_, _ = Get(context.Background(), "org/repo", nil)
	Close("org/repo")
	if !closeCalled {
		t.Error("repoclient.Close not called for Close")
	}
}

func TestRecreate(t *testing.T) {
	var makeCalled, createCalled, initCalled bool
	githubrepoMakeGithubRepo = func(s string) (clients.Repo, error) {
		makeCalled = true
		return nil, nil
	}
	githubrepoCreateGithubRepoClientWithTransport = func(c context.Context, tr http.RoundTripper) clients.RepoClient {
		createCalled = true
		return mockRC{}
	}
	initRepo = func(r clients.Repo, s string) error {
		initCalled = true
		return nil
	}
	close = func() error {
		return nil
	}
	_, _ = Get(context.Background(), "org/repo", nil)
	Close("org/repo")
	makeCalled, createCalled, initCalled = false, false, false
	_, _ = Get(context.Background(), "org/repo", nil)
	if !makeCalled {
		t.Error("githubrepo.MakeGithubRepo not called for new repo")
	}
	if !createCalled {
		t.Error("githubrepo.CreateGithubRepoClientWithTransport not called for new repo")
	}
	if !initCalled {
		t.Error("repoclient.InitRepo not called for new repo")
	}
	Close("org/repo")
}
