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

// Package issue handles creating notification issues for allstar
package issue

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v35/github"
)

const config_Label = "allstar"
const config_Ping = (24 * time.Hour)

const title = "Security Policy violation %v"

func Ensure(ctx context.Context, c *github.Client, owner, repo, policy, text string) error {
	opt := &github.IssueListByRepoOptions{
		State:  "all",
		Labels: []string{config_Label},
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	// TODO: check pagination
	is, _, err := c.Issues.ListByRepo(ctx, owner, repo, opt)
	if err != nil {
		return err
	}
	var issue *github.Issue
	t := fmt.Sprintf(title, policy)
	for _, i := range is {
		if i.GetTitle() == t {
			issue = i
			break
		}
	}
	if issue == nil {
		body := fmt.Sprintf("Security Policy %v is out of compliance, status:\n", policy) + text
		new := &github.IssueRequest{
			Title:  &t,
			Body:   &body,
			Labels: &[]string{config_Label},
		}
		_, _, err := c.Issues.Create(ctx, owner, repo, new)
		return err
	}
	if issue.GetState() == "closed" {
		state := "open"
		update := &github.IssueRequest{
			State: &state,
		}
		if _, _, err := c.Issues.Edit(ctx, owner, repo, issue.GetNumber(), update); err != nil {
			return err
		}
		body := "Re-opening issue, status:\n" + text
		comment := &github.IssueComment{
			Body: &body,
		}
		_, _, err := c.Issues.CreateComment(ctx, owner, repo, issue.GetNumber(), comment)
		return err
	}
	if issue.GetUpdatedAt().Before(time.Now().Add(-1 * config_Ping)) {
		body := "Updating issue after ping interval, status:\n" + text
		comment := &github.IssueComment{
			Body: &body,
		}
		_, _, err := c.Issues.CreateComment(ctx, owner, repo, issue.GetNumber(), comment)
		return err
	}
	return nil
}

func Close(ctx context.Context, c *github.Client, owner, repo, policy string) error {
	opt := &github.IssueListByRepoOptions{
		State:  "all",
		Labels: []string{config_Label},
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	// TODO: check pagination
	is, _, err := c.Issues.ListByRepo(ctx, owner, repo, opt)
	if err != nil {
		return err
	}
	var issue *github.Issue
	t := fmt.Sprintf(title, policy)
	for _, i := range is {
		if i.GetTitle() == t {
			issue = i
			break
		}
	}
	// TODO: above is duplicate, pull into separate function
	if issue.GetState() == "open" {
		body := "In compliance, closing."
		comment := &github.IssueComment{
			Body: &body,
		}
		if _, _, err := c.Issues.CreateComment(ctx, owner, repo, issue.GetNumber(), comment); err != nil {
			return err
		}
		state := "closed"
		update := &github.IssueRequest{
			State: &state,
		}
		if _, _, err := c.Issues.Edit(ctx, owner, repo, issue.GetNumber(), update); err != nil {
			return err
		}
	}
	return nil
}
