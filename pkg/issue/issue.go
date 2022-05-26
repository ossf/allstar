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
	"net/http"
	"time"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/rs/zerolog/log"

	"github.com/google/go-github/v43/github"
)

const issueRepoTitle = "Security Policy violation for repository %q %v"
const sameRepoTitle = "Security Policy violation %v"

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

var configGetAppConfigs func(context.Context, *github.Client, string, string) (*config.OrgConfig, *config.RepoConfig, *config.RepoConfig)

func init() {
	configGetAppConfigs = config.GetAppConfigs
}

func getPolicyIssue(ctx context.Context, issues issues, owner, repo, policy, title, label string) (*github.Issue, error) {
	opt := &github.IssueListByRepoOptions{
		State:  "all",
		Labels: []string{label},
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
	for _, i := range allIssues {
		if i.GetTitle() == title {
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
	return ensure(ctx, c, c.Issues, owner, repo, policy, text)
}

func ensure(ctx context.Context, c *github.Client, issues issues, owner, repo, policy, text string) error {
	issueRepo, title := getIssueRepoTitle(ctx, c, owner, repo, policy)
	label := getIssueLabel(ctx, c, owner, repo)
	issue, err := getPolicyIssue(ctx, issues, owner, issueRepo, policy, title, label)
	if err != nil {
		return err
	}
	if issue == nil {
		oc, _, _ := configGetAppConfigs(ctx, c, owner, repo)
		var footer string
		if oc.IssueFooter == "" {
			footer = operator.GitHubIssueFooter
		} else {
			footer = fmt.Sprintf("%v\n\n%v", oc.IssueFooter, operator.GitHubIssueFooter)
		}
		body := fmt.Sprintf("_This issue was automatically created by [Allstar](https://github.com/ossf/allstar/)._\n\n"+
			"**Security Policy Violation**\n"+
			"%v\n\n---\n\n%v",
			text, footer)
		new := &github.IssueRequest{
			Title:  &title,
			Body:   &body,
			Labels: &[]string{label},
		}
		_, rsp, err := issues.Create(ctx, owner, issueRepo, new)
		if err != nil && rsp != nil && (rsp.StatusCode == http.StatusGone || rsp.StatusCode == http.StatusForbidden) {
			log.Warn().
				Str("org", owner).
				Str("repo", repo).
				Str("area", policy).
				Msg("Action set to issue, but issues are disabled.")
			return nil
		}
		return err
	}
	if issue.GetState() == "closed" {
		state := "open"
		update := &github.IssueRequest{
			State: &state,
		}
		if _, rsp, err := issues.Edit(ctx, owner, issueRepo, issue.GetNumber(), update); err != nil {
			if rsp != nil && (rsp.StatusCode == http.StatusGone || rsp.StatusCode == http.StatusForbidden) {
				log.Warn().
					Str("org", owner).
					Str("repo", repo).
					Str("area", policy).
					Msg("Action set to issue, but issues are disabled.")
				return nil
			}
			return err
		}
		body := "Reopening issue. Status:\n" + text
		comment := &github.IssueComment{
			Body: &body,
		}
		_, _, err := issues.CreateComment(ctx, owner, issueRepo, issue.GetNumber(), comment)
		return err
	}
	if issue.GetUpdatedAt().Before(time.Now().Add(-1 * operator.NoticePingDuration)) {
		body := "Updating issue after ping interval. Status:\n" + text
		comment := &github.IssueComment{
			Body: &body,
		}
		_, rsp, err := issues.CreateComment(ctx, owner, issueRepo, issue.GetNumber(), comment)
		if err != nil && rsp != nil && (rsp.StatusCode == http.StatusGone || rsp.StatusCode == http.StatusForbidden) {
			log.Warn().
				Str("org", owner).
				Str("repo", repo).
				Str("area", policy).
				Msg("Action set to issue, but issues are disabled.")
			return nil
		}
		return err
	}
	return nil
}

// Close ensures that there is not an issue open for the provided repo and
// policy. If open it closes it with a message.
func Close(ctx context.Context, c *github.Client, owner, repo, policy string) error {
	return closeIssue(ctx, c, c.Issues, owner, repo, policy)
}

func closeIssue(ctx context.Context, c *github.Client, issues issues, owner, repo, policy string) error {
	issueRepo, title := getIssueRepoTitle(ctx, c, owner, repo, policy)
	label := getIssueLabel(ctx, c, owner, repo)
	issue, err := getPolicyIssue(ctx, issues, owner, issueRepo, policy, title, label)
	if err != nil {
		return err
	}
	if issue.GetState() == "open" {
		body := "Policy is now in compliance. Closing issue."
		comment := &github.IssueComment{
			Body: &body,
		}
		if _, rsp, err := issues.CreateComment(ctx, owner, issueRepo, issue.GetNumber(), comment); err != nil {
			if rsp != nil && (rsp.StatusCode == http.StatusGone || rsp.StatusCode == http.StatusForbidden) {
				log.Warn().
					Str("org", owner).
					Str("repo", repo).
					Str("area", policy).
					Msg("Action set to issue, but issues are disabled.")
				return nil
			}
			return err
		}
		state := "closed"
		update := &github.IssueRequest{
			State: &state,
		}
		if _, _, err := issues.Edit(ctx, owner, issueRepo, issue.GetNumber(), update); err != nil {
			return err
		}
	}
	return nil
}

func getIssueLabel(ctx context.Context, c *github.Client, owner, repo string) string {
	label := operator.GitHubIssueLabel
	oc, orc, rc := configGetAppConfigs(ctx, c, owner, repo)
	if len(oc.IssueLabel) > 0 {
		label = oc.IssueLabel
	}
	if len(orc.IssueLabel) > 0 {
		label = orc.IssueLabel
	}
	if len(rc.IssueLabel) > 0 {
		label = rc.IssueLabel
	}
	return label
}

func getIssueRepoTitle(ctx context.Context, c *github.Client, owner, repo, policy string) (string, string) {
	oc, _, _ := configGetAppConfigs(ctx, c, owner, repo)
	if len(oc.IssueRepo) > 0 {
		return oc.IssueRepo, fmt.Sprintf(issueRepoTitle, repo, policy)
	}
	return repo, fmt.Sprintf(sameRepoTitle, policy)
}
