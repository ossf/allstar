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
	"io"
	"net/http"
	"testing"
	time "time"

	"github.com/ossf/scorecard/v5/clients"
	"github.com/ossf/scorecard/v5/clients/localdir"
)

var initRepo func(clients.Repo, string, int) error
var close func() error

type mockRC struct{}

func (m mockRC) InitRepo(r clients.Repo, s string, i int) error {
	return initRepo(r, s, i)
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

func (m mockRC) LocalPath() (string, error) {
	return "", nil
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

func (m mockRC) GetCreatedAt() (time.Time, error) {
	return time.Now(), nil
}

func (m mockRC) GetDefaultBranch() (*clients.BranchRef, error) {
	return nil, nil
}

func (m mockRC) GetOrgRepoClient(context.Context) (clients.RepoClient, error) {
	return nil, nil
}

func (m mockRC) GetDefaultBranchName() (string, error) {
	return "", nil
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

func (m mockRC) ListLicenses() ([]clients.License, error) {
	return nil, nil
}

func (m mockRC) SearchCommits(request clients.SearchCommitsOptions) ([]clients.Commit, error) {
	return nil, nil
}

func (m mockRC) Close() error {
	return close()
}

func (m mockRC) GetFileReader(filename string) (io.ReadCloser, error) {
	return nil, nil
}

func TestGetNew(t *testing.T) {
	var makeCalled, createCalled, initCalled bool
	githubrepoMakeGitHubRepo = func(s string) (clients.Repo, error) {
		makeCalled = true
		return nil, nil
	}
	githubrepoCreateGitHubRepoClientWithTransport = func(c context.Context, tr http.RoundTripper) clients.RepoClient {
		createCalled = true
		return mockRC{}
	}
	initRepo = func(r clients.Repo, s string, i int) error {
		initCalled = true
		return nil
	}
	close = func() error {
		return nil
	}
	_, _ = Get(context.Background(), "org/repo", false, nil)
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
	githubrepoMakeGitHubRepo = func(s string) (clients.Repo, error) {
		makeCalled = true
		return nil, nil
	}
	githubrepoCreateGitHubRepoClientWithTransport = func(c context.Context, tr http.RoundTripper) clients.RepoClient {
		createCalled = true
		return mockRC{}
	}
	initRepo = func(r clients.Repo, s string, i int) error {
		initCalled = true
		return nil
	}
	close = func() error {
		return nil
	}
	_, _ = Get(context.Background(), "org/repo", false, nil)
	makeCalled, createCalled, initCalled = false, false, false
	_, _ = Get(context.Background(), "org/repo", false, nil)
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
	githubrepoMakeGitHubRepo = func(s string) (clients.Repo, error) {
		return nil, nil
	}
	githubrepoCreateGitHubRepoClientWithTransport = func(c context.Context, tr http.RoundTripper) clients.RepoClient {
		return mockRC{}
	}
	initRepo = func(r clients.Repo, s string, i int) error {
		return nil
	}
	close = func() error {
		closeCalled = true
		return nil
	}
	_, _ = Get(context.Background(), "org/repo", false, nil)
	Close("org/repo")
	if !closeCalled {
		t.Error("repoclient.Close not called for Close")
	}
}

func TestRecreate(t *testing.T) {
	var makeCalled, createCalled, initCalled bool
	githubrepoMakeGitHubRepo = func(s string) (clients.Repo, error) {
		makeCalled = true
		return nil, nil
	}
	githubrepoCreateGitHubRepoClientWithTransport = func(c context.Context, tr http.RoundTripper) clients.RepoClient {
		createCalled = true
		return mockRC{}
	}
	initRepo = func(r clients.Repo, s string, i int) error {
		initCalled = true
		return nil
	}
	close = func() error {
		return nil
	}
	_, _ = Get(context.Background(), "org/repo", false, nil)
	Close("org/repo")
	makeCalled, createCalled, initCalled = false, false, false
	_, _ = Get(context.Background(), "org/repo", false, nil)
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

// as we cannot interact with the GitHub API, this is a simple version of createLocal
// which just fetches a public repo for local testing
func _testCreateLocal(ctx context.Context, fullRepo string) (*ScClient, error) {
	localPath, gitRepo, err := checkoutRepo(fullRepo, "")
	if err != nil {
		return nil, err
	}

	scr, err := localdir.MakeLocalDirRepo(localPath)
	if err != nil {
		return nil, err
	}
	return &ScClient{
		ScRepo:    scr,
		localPath: localPath,
		gitRepo:   gitRepo,
	}, nil
}


func TestLocal(t *testing.T) {
	createLocal = _testCreateLocal
	repo := "go-git/go-git"
	scc, err := Get(context.Background(), repo, true, nil)
	if err != nil {
		t.Fatalf("Get error: %s", err)
	}
	branches, err := scc.FetchBranches()
	if err != nil {
		t.Fatalf("FetchBranches error: %s", err)
	}
	err = scc.SwitchLocalBranch(branches[0])
	if err != nil {
		t.Fatalf("SwitchLocalBranch error: %s", err)
	}

	// fetch again and make sure the reference is the same
	sccB, err := Get(context.Background(), repo, true, nil)
	if err != nil {
		t.Fatalf("Get error: %s", err)
	}
	if scc != sccB {
		t.Fatalf("Get error: unexpected different reference returned")
	}

	Close(repo)
}
