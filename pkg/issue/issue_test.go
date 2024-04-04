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

package issue

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/contentful/allstar/pkg/config"
	"github.com/contentful/allstar/pkg/config/operator"

	"github.com/google/go-github/v59/github"
)

var listByRepo func(context.Context, string, string,
	*github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error)
var create func(context.Context, string, string, *github.IssueRequest) (
	*github.Issue, *github.Response, error)
var edit func(context.Context, string, string, int, *github.IssueRequest) (
	*github.Issue, *github.Response, error)
var createComment func(context.Context, string, string, int,
	*github.IssueComment) (*github.IssueComment, *github.Response, error)

type mockIssues struct{}

func (m mockIssues) ListByRepo(ctx context.Context, owner string, repo string,
	opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
	return listByRepo(ctx, owner, repo, opts)
}

func (m mockIssues) Create(ctx context.Context, owner string, repo string,
	issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
	return create(ctx, owner, repo, issue)
}

func (m mockIssues) Edit(ctx context.Context, owner string, repo string,
	number int, issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
	return edit(ctx, owner, repo, number, issue)
}

func (m mockIssues) CreateComment(ctx context.Context, owner string, repo string,
	number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	return createComment(ctx, owner, repo, number, comment)
}

func setShouldPerform(b bool) {
	scheduleShouldPerform = func(*config.ScheduleConfig) bool {
		return b
	}
}

func TestEnsure(t *testing.T) {
	//issueTitle := fmt.Sprintf(sameRepoTitle, "thispolicy")
	issueTitle := "Security Policy violation thispolicy"
	issueTitleOtherRepo := "Security Policy violation for repository \"\" thispolicy"
	closed := "closed"
	open := "open"
	body := "_This issue was automatically created by [Allstar](https://github.com/contentful/allstar/)._\n\n**Security Policy Violation**\nStatus text\n\n---\n\n<!-- Edit section #updates --><!-- Current result text hash: 1ab61918ea1b7d10e20db2b40287c1a265a1617b998d87b28579a4462b2efac2 --><!-- Edit section #updates -->\nThis issue will auto resolve when the policy is in compliance.\n\nIssue created by Allstar. See https://github.com/contentful/allstar/ for more information. For questions specific to the repository, please contact the owner or maintainer."
	bodyOtherRepo := "_This issue was automatically created by [Allstar](https://github.com/contentful/allstar/) and refers to [/](https://github.com//)._\n\n**Security Policy Violation**\nStatus text\n\n---\n\n<!-- Edit section #updates --><!-- Current result text hash: 1ab61918ea1b7d10e20db2b40287c1a265a1617b998d87b28579a4462b2efac2 --><!-- Edit section #updates -->\nThis issue will auto resolve when the policy is in compliance.\n\nIssue created by Allstar. See https://github.com/contentful/allstar/ for more information. For questions specific to the repository, please contact the owner or maintainer."
	configGetAppConfigs = func(context.Context, *github.Client, string, string) (*config.OrgConfig, *config.RepoConfig, *config.RepoConfig) {
		return &config.OrgConfig{}, &config.RepoConfig{}, &config.RepoConfig{}
	}
	t.Run("NoIssue", func(t *testing.T) {
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return make([]*github.Issue, 0), &github.Response{NextPage: 0}, nil
		}
		createCalled := false
		create = func(ctx context.Context, owner string, repo string,
			issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
			if *issue.Title != issueTitle {
				t.Errorf("Unexpected title: %q expect: %q", issue.GetTitle(), issueTitle)
			}
			if (*issue.Labels)[0] != operator.GitHubIssueLabel {
				t.Errorf("Unexpected label: %v", (*issue.Labels)[0])
			}
			if *issue.Body != body {
				t.Errorf("Unexpected body: %q expect: %q", issue.GetBody(), body)
			}
			createCalled = true
			return nil, nil, nil
		}
		edit = nil
		createComment = nil
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "Status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if createCalled != true {
			t.Error("Expected issue to be created")
		}
	})
	t.Run("NoIssueInAnotherRepo", func(t *testing.T) {
		configGetAppConfigs = func(context.Context, *github.Client, string, string) (*config.OrgConfig, *config.RepoConfig, *config.RepoConfig) {
			return &config.OrgConfig{IssueRepo: "issuerepo"}, &config.RepoConfig{}, &config.RepoConfig{}
		}
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return make([]*github.Issue, 0), &github.Response{NextPage: 0}, nil
		}
		createCalled := false
		create = func(ctx context.Context, owner string, repo string,
			issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
			if *issue.Title != issueTitleOtherRepo {
				t.Errorf("Unexpected title: %q expect: %q", issue.GetTitle(), issueTitleOtherRepo)
			}
			if (*issue.Labels)[0] != operator.GitHubIssueLabel {
				t.Errorf("Unexpected label: %v", (*issue.Labels)[0])
			}
			if *issue.Body != bodyOtherRepo {
				t.Errorf("Unexpected body: %q expect: %q", issue.GetBody(), bodyOtherRepo)
			}
			createCalled = true
			return nil, nil, nil
		}
		edit = nil
		createComment = nil
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "Status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if createCalled != true {
			t.Error("Expected issue to be created")
		}
	})
	t.Run("NoIssueWithFooter", func(t *testing.T) {
		configGetAppConfigs = func(context.Context, *github.Client, string, string) (*config.OrgConfig, *config.RepoConfig, *config.RepoConfig) {
			return &config.OrgConfig{IssueFooter: "CustomFooter"}, &config.RepoConfig{}, &config.RepoConfig{}
		}
		bodyWithFooter := "_This issue was automatically created by [Allstar](https://github.com/contentful/allstar/)._\n\n**Security Policy Violation**\nStatus text\n\n---\n\n<!-- Edit section #updates --><!-- Current result text hash: 1ab61918ea1b7d10e20db2b40287c1a265a1617b998d87b28579a4462b2efac2 --><!-- Edit section #updates -->\nCustomFooter\n\nThis issue will auto resolve when the policy is in compliance.\n\nIssue created by Allstar. See https://github.com/contentful/allstar/ for more information. For questions specific to the repository, please contact the owner or maintainer."
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return make([]*github.Issue, 0), &github.Response{NextPage: 0}, nil
		}
		createCalled := false
		create = func(ctx context.Context, owner string, repo string,
			issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
			if *issue.Title != issueTitle {
				t.Errorf("Unexpected title: %q expect: %q", issue.GetTitle(), issueTitle)
			}
			if (*issue.Labels)[0] != operator.GitHubIssueLabel {
				t.Errorf("Unexpected label: %v", (*issue.Labels)[0])
			}
			if *issue.Body != bodyWithFooter {
				t.Errorf("Unexpected body: %q expect: %q", issue.GetBody(), bodyWithFooter)
			}
			createCalled = true
			return nil, nil, nil
		}
		edit = nil
		createComment = nil
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "Status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if createCalled != true {
			t.Error("Expected issue to be created")
		}
	})
	t.Run("ClosedIssue", func(t *testing.T) {
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return []*github.Issue{
				&github.Issue{
					Title: &issueTitle,
					State: &closed,
				},
			}, &github.Response{NextPage: 0}, nil
		}
		create = nil
		editCalled := false
		edit = func(ctx context.Context, owner string, repo string, number int,
			issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
			if issue.GetState() != "open" {
				t.Errorf("Unexpected state: %v", issue.GetState())
			}
			editCalled = true
			return nil, nil, nil
		}
		commentCalled := false
		createComment = func(ctx context.Context, owner string, repo string,
			number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
			if !strings.HasPrefix(comment.GetBody(), "Reopening issue") {
				t.Errorf("Unexpected comment: %v", comment.GetBody())
			}
			commentCalled = true
			return nil, nil, nil
		}
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "Status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if editCalled != true {
			t.Error("Expected issue to be re-opened")
		}
		if commentCalled != true {
			t.Error("Expected comment to be left")
		}
	})
	t.Run("ClosedIssueUpdatedText", func(t *testing.T) {
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return []*github.Issue{
				&github.Issue{
					Title: &issueTitle,
					State: &closed,
					Body:  &body,
				},
			}, &github.Response{NextPage: 0}, nil
		}
		create = nil
		editCalled := false
		edit = func(ctx context.Context, owner string, repo string, number int,
			issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
			if issue.GetState() != "open" {
				t.Errorf("Unexpected state: %v", issue.GetState())
			}
			if !strings.Contains(issue.GetBody(), "<!-- Current result text hash: 3079c555ed4ca1a5d828505dd39ed455313c07451755e380c36ee48a359038e9 -->") {
				t.Errorf("Unexpected issue edit body (missing hash): %v", issue.GetBody())
			}
			if !strings.Contains(issue.GetBody(), "Click here to see the latest update") {
				t.Errorf("Unexpected issue edit body (missing link to update): %v", issue.GetBody())
			}
			editCalled = true
			return nil, nil, nil
		}
		commentCalled := false
		createComment = func(ctx context.Context, owner string, repo string,
			number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
			if !strings.HasPrefix(comment.GetBody(), "The policy result has been updated") {
				t.Errorf("Unexpected comment: %v", comment.GetBody())
			}
			commentCalled = true
			return nil, nil, nil
		}
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "New status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if editCalled != true {
			t.Error("Expected issue to be re-opened")
		}
		if commentCalled != true {
			t.Error("Expected comment to be left")
		}
	})
	t.Run("OpenFreshIssue", func(t *testing.T) {
		now := github.Timestamp{Time: time.Now()}

		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return []*github.Issue{
				&github.Issue{
					Title:     &issueTitle,
					State:     &open,
					UpdatedAt: &now,
				},
			}, &github.Response{NextPage: 0}, nil
		}
		// Expect to not call nil functions
		create = nil
		edit = nil
		createComment = nil
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "Status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
	t.Run("OpenStaleIssue", func(t *testing.T) {
		stale := github.Timestamp{Time: time.Now().Add(-10 * operator.NoticePingDuration)}
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return []*github.Issue{
				&github.Issue{
					Title:     &issueTitle,
					State:     &open,
					UpdatedAt: &stale,
				},
			}, &github.Response{NextPage: 0}, nil
		}
		commentCalled := false
		createComment = func(ctx context.Context, owner string, repo string,
			number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
			if !strings.HasPrefix(comment.GetBody(), "Updating issue") {
				t.Errorf("Unexpected comment: %v", comment.GetBody())
			}
			commentCalled = true
			return nil, nil, nil
		}
		// Expect to not call nil functions
		create = nil
		edit = nil
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "Status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if commentCalled != true {
			t.Error("Expected comment to be left")
		}
	})
	t.Run("NoIssueScheduleBlocksCreate", func(t *testing.T) {
		setShouldPerform(false)
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return make([]*github.Issue, 0), &github.Response{NextPage: 0}, nil
		}
		createCalled := false
		create = func(ctx context.Context, owner string, repo string,
			issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
			if *issue.Title != issueTitle {
				t.Errorf("Unexpected title: %q expect: %q", issue.GetTitle(), issueTitle)
			}
			if (*issue.Labels)[0] != operator.GitHubIssueLabel {
				t.Errorf("Unexpected label: %v", (*issue.Labels)[0])
			}
			if *issue.Body != body {
				t.Errorf("Unexpected body: %q expect: %q", issue.GetBody(), body)
			}
			createCalled = true
			return nil, nil, nil
		}
		edit = nil
		createComment = nil
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "Status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if createCalled != false {
			t.Error("Expected no issue to be created")
		}
	})
	t.Run("NoIssueScheduleAllowsCreate", func(t *testing.T) {
		configGetAppConfigs = func(context.Context, *github.Client, string, string) (*config.OrgConfig, *config.RepoConfig, *config.RepoConfig) {
			return &config.OrgConfig{}, &config.RepoConfig{}, &config.RepoConfig{}
		}
		setShouldPerform(true)
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return make([]*github.Issue, 0), &github.Response{NextPage: 0}, nil
		}
		createCalled := false
		create = func(ctx context.Context, owner string, repo string,
			issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
			if *issue.Title != issueTitle {
				t.Errorf("Unexpected title: %q expect: %q", issue.GetTitle(), issueTitle)
			}
			if (*issue.Labels)[0] != operator.GitHubIssueLabel {
				t.Errorf("Unexpected label: %v", (*issue.Labels)[0])
			}
			if *issue.Body != body {
				t.Errorf("Unexpected body: %q expect: %q", issue.GetBody(), body)
			}
			createCalled = true
			return nil, nil, nil
		}
		edit = nil
		createComment = nil
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "Status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if createCalled != true {
			t.Error("Expected issue to be created")
		}
	})
	t.Run("OpenStaleIssueScheduleAllowsPing", func(t *testing.T) {
		setShouldPerform(true)
		stale := github.Timestamp{Time: time.Now().Add(-10 * operator.NoticePingDuration)}
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return []*github.Issue{
				&github.Issue{
					Title:     &issueTitle,
					State:     &open,
					UpdatedAt: &stale,
				},
			}, &github.Response{NextPage: 0}, nil
		}
		commentCalled := false
		createComment = func(ctx context.Context, owner string, repo string,
			number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
			if !strings.HasPrefix(comment.GetBody(), "Updating issue") {
				t.Errorf("Unexpected comment: %v", comment.GetBody())
			}
			commentCalled = true
			return nil, nil, nil
		}
		// Expect to not call nil functions
		create = nil
		edit = nil
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "Status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if commentCalled != true {
			t.Error("Expected comment to be left")
		}
	})
	t.Run("OpenStaleIssueScheduleBlocksPing", func(t *testing.T) {
		setShouldPerform(false)
		stale := github.Timestamp{Time: time.Now().Add(-10 * operator.NoticePingDuration)}
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return []*github.Issue{
				&github.Issue{
					Title:     &issueTitle,
					State:     &open,
					UpdatedAt: &stale,
				},
			}, &github.Response{NextPage: 0}, nil
		}
		commentCalled := false
		createComment = func(ctx context.Context, owner string, repo string,
			number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
			if !strings.HasPrefix(comment.GetBody(), "Updating issue") {
				t.Errorf("Unexpected comment: %v", comment.GetBody())
			}
			commentCalled = true
			return nil, nil, nil
		}
		// Expect to not call nil functions
		create = nil
		edit = nil
		err := ensure(context.Background(), nil, mockIssues{}, "", "", "thispolicy", "Status text")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if commentCalled != false {
			t.Error("Expected no comment to be left")
		}
	})
}

func TestClose(t *testing.T) {
	issueTitle := fmt.Sprintf(sameRepoTitle, "thispolicy")
	configGetAppConfigs = func(context.Context, *github.Client, string, string) (*config.OrgConfig, *config.RepoConfig, *config.RepoConfig) {
		return &config.OrgConfig{}, &config.RepoConfig{}, &config.RepoConfig{}
	}
	t.Run("NoIssue", func(t *testing.T) {
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return make([]*github.Issue, 0), &github.Response{NextPage: 0}, nil
		}
		// Expect to not call nil functions
		createComment = nil
		edit = nil
		err := closeIssue(context.Background(), nil, mockIssues{}, "", "", "thispolicy")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
	t.Run("ClosedIssue", func(t *testing.T) {
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			return []*github.Issue{
				&github.Issue{
					Title: &issueTitle,
				},
			}, &github.Response{NextPage: 0}, nil
		}
		// Expect to not call nil functions
		createComment = nil
		edit = nil
		err := closeIssue(context.Background(), nil, mockIssues{}, "", "", "thispolicy")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
	t.Run("OpenIssue", func(t *testing.T) {
		listByRepo = func(ctx context.Context, owner string, repo string,
			opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
			open := "open"
			return []*github.Issue{
				&github.Issue{
					Title: &issueTitle,
					State: &open,
				},
			}, &github.Response{NextPage: 0}, nil
		}
		commentCalled := false
		createComment = func(ctx context.Context, owner string, repo string,
			number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
			if comment.GetBody() != "Policy is now in compliance. Closing issue." {
				t.Errorf("Unexpected comment: %v", comment.GetBody())
			}
			commentCalled = true
			return nil, nil, nil
		}
		editCalled := false
		edit = func(ctx context.Context, owner string, repo string, number int,
			issue *github.IssueRequest) (*github.Issue, *github.Response, error) {
			if issue.GetState() != "closed" {
				t.Errorf("Unexpected state: %v", issue.GetState())
			}
			editCalled = true
			return nil, nil, nil
		}
		err := closeIssue(context.Background(), nil, mockIssues{}, "", "", "thispolicy")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if commentCalled != true {
			t.Error("Expected comment to be left")
		}
		if editCalled != true {
			t.Error("Expected issue to be closed")
		}
	})
}

func TestLabel(t *testing.T) {
	tests := []struct {
		Name         string
		OrgLabel     string
		OrgRepoLabel string
		RepoLabel    string
		Expect       string
	}{
		{
			Name:   "None",
			Expect: operator.GitHubIssueLabel,
		},
		{
			Name:     "Org",
			OrgLabel: "orglabel",
			Expect:   "orglabel",
		},
		{
			Name:      "Repo1",
			RepoLabel: "repolabel",
			Expect:    "repolabel",
		},
		{
			Name:      "Repo2",
			OrgLabel:  "orglabel",
			RepoLabel: "repolabel",
			Expect:    "repolabel",
		},
		{
			Name:         "OrgRepo1",
			OrgRepoLabel: "repolabel",
			Expect:       "repolabel",
		},
		{
			Name:         "OrgRepo2",
			OrgLabel:     "orglabel",
			OrgRepoLabel: "repolabel",
			Expect:       "repolabel",
		},
		{
			Name:         "Repo3",
			OrgLabel:     "orglabel",
			OrgRepoLabel: "orgrepolabel",
			RepoLabel:    "repolabel",
			Expect:       "repolabel",
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configGetAppConfigs = func(context.Context, *github.Client, string, string) (*config.OrgConfig, *config.RepoConfig, *config.RepoConfig) {
				return &config.OrgConfig{IssueLabel: test.OrgLabel}, &config.RepoConfig{IssueLabel: test.OrgRepoLabel}, &config.RepoConfig{IssueLabel: test.RepoLabel}
			}
			got := getIssueLabel(context.Background(), nil, "", "")
			if got != test.Expect {
				t.Errorf("Unexpected label. Want: %q Got: %q", test.Expect, got)
			}
		})
	}
}

func TestRepoTitle(t *testing.T) {
	tests := []struct {
		Name      string
		Repo      string
		Policy    string
		IssueRepo string
		ExpRepo   string
		ExpTitle  string
	}{
		{
			Name:     "Not set",
			Repo:     "testrepo",
			Policy:   "testpolicy",
			ExpRepo:  "testrepo",
			ExpTitle: "Security Policy violation testpolicy",
		},
		{
			Name:      "Set",
			Repo:      "testrepo",
			Policy:    "testpolicy",
			IssueRepo: "issuerepo",
			ExpRepo:   "issuerepo",
			ExpTitle:  "Security Policy violation for repository \"testrepo\" testpolicy",
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configGetAppConfigs = func(context.Context, *github.Client, string, string) (*config.OrgConfig, *config.RepoConfig, *config.RepoConfig) {
				return &config.OrgConfig{IssueRepo: test.IssueRepo}, nil, nil
			}
			gotRepo, gotTitle := getIssueRepoTitle(context.Background(), nil, "", test.Repo, test.Policy)
			if gotRepo != test.ExpRepo {
				t.Errorf("Unexpected Issue Repo. Want: %q Got: %q", test.ExpRepo, gotRepo)
			}
			if gotTitle != test.ExpTitle {
				t.Errorf("Unexpected Issue Title. Want: %q Got: %q", test.ExpTitle, gotTitle)
			}
		})
	}
}
