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

// Package issue handles creating notification GitHub issues for Allstar
package issue

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/ossf/allstar/pkg/config/operator"
)

const title = "Security Policy violation %v"

type issues interface {
	ListByRepo(context.Context, string, string, *github.IssueListByRepoOptions) (
		[]*github.Issue, *github.Response, error)
	Create(context.Context, string, string, *github.IssueRequest) (
		*github.Issue, *github.Response, error)
	Edit(context.Context, string, string, int, *github.IssueRequest) (
		*github.Issue, *github.Response, error)
	CreateComment(context.Context, string, string, int, *github.IssueComment) (
		*github.IssueComment, *github.Response, error)
}

func getPolicyIssue(ctx context.Context, issues issues, owner, repo, policy string) (*github.Issue, error) {
	opt := &github.IssueListByRepoOptions{
		State:  "all",
		Labels: []string{operator.GitHubIssueLabel},
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	var allIssues []*github.Issue
	for {
		is, resp, err := issues.ListByRepo(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		allIssues = append(allIssues, is...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	var issue *github.Issue
	t := fmt.Sprintf(title, policy)
	for _, i := range allIssues {
		if i.GetTitle() == t {
			issue = i
			break
		}
	}
	return issue, nil
}

// Ensure ensures an issue exists and is open for the provided repo and
// policy. If opening, re-opening, or pinging an issue, the provided text will
// be included.
func Ensure(ctx context.Context, c *github.Client, owner, repo, policy, text string) error {
	return ensure(ctx, c.Issues, owner, repo, policy, text)
}

func ensure(ctx context.Context, issues issues, owner, repo, policy, text string) error {
	issue, err := getPolicyIssue(ctx, issues, owner, repo, policy)
	if err != nil {
		return err
	}
	if issue == nil {
		body := fmt.Sprintf("Allstar has detected that this repository’s %v security policy is out of compliance. Status:\n%v\n\n%v",
			policy, text, operator.GitHubIssueFooter)
		t := fmt.Sprintf(title, policy)
		new := &github.IssueRequest{
			Title:  &t,
			Body:   &body,
			Labels: &[]string{operator.GitHubIssueLabel},
		}
		_, _, err := issues.Create(ctx, owner, repo, new)
		return err
	}
	if issue.GetState() == "closed" {
		state := "open"
		update := &github.IssueRequest{
			State: &state,
		}
		if _, _, err := issues.Edit(ctx, owner, repo, issue.GetNumber(), update); err != nil {
			return err
		}
		body := "Reopening issue. Status:\n" + text
		comment := &github.IssueComment{
			Body: &body,
		}
		_, _, err := issues.CreateComment(ctx, owner, repo, issue.GetNumber(), comment)
		return err
	}
	if issue.GetUpdatedAt().Before(time.Now().Add(-1 * operator.NoticePingDuration)) {
		body := "Updating issue after ping interval. Status:\n" + text
		comment := &github.IssueComment{
			Body: &body,
		}
		_, _, err := issues.CreateComment(ctx, owner, repo, issue.GetNumber(), comment)
		return err
	}
	return nil
}

// Close ensures that there is not an issue open for the provided repo and
// policy. If open it closes it with a message.
func Close(ctx context.Context, c *github.Client, owner, repo, policy string) error {
	return closeIssue(ctx, c.Issues, owner, repo, policy)
}

func closeIssue(ctx context.Context, issues issues, owner, repo, policy string) error {
	issue, err := getPolicyIssue(ctx, issues, owner, repo, policy)
	if err != nil {
		return err
	}
	if issue.GetState() == "open" {
		body := "Policy is now in compliance. Closing issue."
		comment := &github.IssueComment{
			Body: &body,
		}
		if _, _, err := issues.CreateComment(ctx, owner, repo, issue.GetNumber(), comment); err != nil {
			return err
		}
		state := "closed"
		update := &github.IssueRequest{
			State: &state,
		}
		if _, _, err := issues.Edit(ctx, owner, repo, issue.GetNumber(), update); err != nil {
			return err
		}
	}
	return nil
}
