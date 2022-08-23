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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/config/operator"
	"github.com/ossf/allstar/pkg/config/schedule"
	"github.com/rs/zerolog/log"

	"github.com/google/go-github/v43/github"
)

const issueRepoTitle = "Security Policy violation for repository %q %v"
const sameRepoTitle = "Security Policy violation %v"

const issueSectionHeaderFormat = "<!-- Edit section #%s -->"
const resultTextHashCommentFormat = "<!-- Current result text hash: %s -->"
const updateWarningFormat = "\n%s\n:warning: There is an updated version of this policy result! [Click here to see the latest update](%s)\n\n---\n\n"
const updateSectionName = "updates"

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
var scheduleShouldPerform func(*config.ScheduleConfig) bool

func init() {
	configGetAppConfigs = config.GetAppConfigs
	scheduleShouldPerform = schedule.ShouldPerform
}

func getPolicyIssue(ctx context.Context, issues issues, owner, repo, policy, title, label string) (*github.Issue, error) {
	opt := &github.IssueListByRepoOptions{
		State:  "all",
		Labels: []string{label},
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	for {
		is, resp, err := issues.ListByRepo(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		for _, i := range is {
			if i.GetTitle() == title {
				return i, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return nil, nil
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
	oc, orc, rc := configGetAppConfigs(ctx, c, owner, repo)
	osc := schedule.MergeSchedules(oc.Schedule, orc.Schedule, rc.Schedule)
	shouldPing := scheduleShouldPerform(osc)
	// Hash text for update checking
	h := sha256.New()
	if _, err := h.Write([]byte(text)); err != nil {
		return err
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if issue == nil {
		if !shouldPing {
			return nil
		}
		var footer string
		if oc.IssueFooter == "" {
			footer = operator.GitHubIssueFooter
		} else {
			footer = fmt.Sprintf("%v\n\n%v", oc.IssueFooter, operator.GitHubIssueFooter)
		}
		body := createIssueBody(owner, repo, text, hash, footer, issueRepo == repo)
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
	// Check if current-version issue is not up to date
	if !strings.Contains(issue.GetBody(), hash) && hasIssueSection(issue.GetBody(), updateSectionName) {
		// Comment update and update issue body
		commentBody := fmt.Sprintf("The policy result has been updated.\n\n---\n\n%s", text)
		comment, _, err := issues.CreateComment(ctx, owner, repo, issue.GetNumber(), &github.IssueComment{
			Body: &commentBody,
		})
		if err != nil {
			return fmt.Errorf("while updating issue: creating comment: %w", err)
		}
		updateWarning := fmt.Sprintf(updateWarningFormat, fmt.Sprintf(resultTextHashCommentFormat, hash), comment.GetHTMLURL())
		newBody, ok := updateIssueSection(issue.GetBody(), updateSectionName, updateWarning)
		if !ok {
			// This shouldn't occur because hasIssueSection was true
			log.Error().
				Str("org", owner).
				Str("repo", repo).
				Str("area", policy).
				Int("issueNumber", issue.GetNumber()).
				Msg("Unexpectedly failed to update issue update section.")
			return nil
		}
		// Ensure issue is open as well
		state := "open"
		_, _, err = issues.Edit(ctx, owner, repo, issue.GetNumber(), &github.IssueRequest{
			State: &state,
			Body:  &newBody,
		})
		if err != nil {
			return fmt.Errorf("while updating issue %d: editing body: %w", issue.GetNumber(), err)
		}
		return nil
	}
	// If should not ping, don't continue. Below here is:
	// - Reopen (& ping) if closed & not passing
	// - Ping after interval
	// Note that the ping above (on edit) remains.
	if !shouldPing {
		return nil
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
		body := fmt.Sprintf("Reopening issue. See its status below.\n\n---\n\n%s", text)
		comment := &github.IssueComment{
			Body: &body,
		}
		_, _, err := issues.CreateComment(ctx, owner, issueRepo, issue.GetNumber(), comment)
		return err
	}
	if issue.GetUpdatedAt().Before(time.Now().Add(-1 * operator.NoticePingDuration)) {
		body := fmt.Sprintf("Updating issue after ping interval. See its status below.\n\n---\n\n%s", text)
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

func createIssueBody(owner, repo, text, hash, footer string, isIssueRepo bool) string {
	var refersTo string
	if !isIssueRepo {
		ownerRepo := fmt.Sprintf("%s/%s", owner, repo)
		refersTo = fmt.Sprintf(" and refers to [%s](https://github.com/%s)", ownerRepo, ownerRepo)
	}
	editHeader := issueSectionHeader(updateSectionName)
	return fmt.Sprintf("_This issue was automatically created by [Allstar](https://github.com/ossf/allstar/)%s._\n\n**Security Policy Violation**\n"+
		"%v\n\n---\n\n%s%s%s\n%v",
		refersTo, text, editHeader, fmt.Sprintf(resultTextHashCommentFormat, hash), editHeader, footer)
}

func issueSectionHeader(sectionName string) string {
	return fmt.Sprintf(issueSectionHeaderFormat, sectionName)
}

func hasIssueSection(body, sectionName string) bool {
	return strings.Count(body, issueSectionHeader(sectionName)) == 2
}

func updateIssueSection(body, sectionName, editText string) (string, bool) {
	header := issueSectionHeader(sectionName)
	s := strings.Split(body, header)
	if len(s) != 3 {
		return body, false
	}
	return strings.Join([]string{s[0], header, editText, header, s[2]}, ""), true
}
