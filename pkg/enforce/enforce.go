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

// Package enforce is a central engine to Allstar that contains various
// enforcement logic.
package enforce

import (
	"context"
	"time"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/ghclients"
	"github.com/ossf/allstar/pkg/issue"
	"github.com/ossf/allstar/pkg/policies"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v43/github"
	"github.com/rs/zerolog/log"
)

var policiesGetPolicies func() []policydef.Policy
var issueEnsure func(ctx context.Context, c *github.Client, owner, repo, policy, text string) error
var issueClose func(ctx context.Context, c *github.Client, owner, repo, policy string) error

func init() {
	policiesGetPolicies = policies.GetPolicies
	issueEnsure = issue.Ensure
	issueClose = issue.Close
}

// EnforceAll iterates through all available installations and repos Allstar
// has access to and runs policies on those repos. It is meant to be a
// reconcilation job to check repos which a webhook event may have been lost.
//
// TBD: determine if this should remain exported, or if it will only be called
// from EnforceJob.
func EnforceAll(ctx context.Context, ghc *ghclients.GHClients) error {
	var repoCount int
	ac, err := ghc.Get(0)
	if err != nil {
		return err
	}
	var insts []*github.Installation
	opt := &github.ListOptions{
		PerPage: 100,
	}
	for {
		is, resp, err := ac.Apps.ListInstallations(ctx, opt)
		if err != nil {
			return err
		}
		insts = append(insts, is...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	log.Info().
		Str("area", "bot").
		Int("count", len(insts)).
		Msg("Enforcing policies on installations.")
	for _, i := range insts {
		ic, err := ghc.Get(*i.ID)
		if err != nil {
			log.Error().
				Err(err).
				Int64("instId", *i.ID).
				Msg("Unexpected error getting installation client.")
			continue
		}
		var repos []*github.Repository
		opt := &github.ListOptions{
			PerPage: 100,
		}
		err = nil
		for {
			var rs *github.ListRepositories
			var resp *github.Response
			rs, resp, err = ic.Apps.ListRepos(ctx, opt)
			if err != nil {
				break
			}
			repos = append(repos, rs.Repositories...)
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
		if err != nil {
			log.Error().
				Err(err).
				Msg("Unexpected error listing installation repos.")
			continue
		}
		err = nil
		log.Info().
			Str("area", "bot").
			Int64("id", *i.ID).
			Int("count", len(repos)).
			Msg("Enforcing policies on repos of installation.")
		repoCount = repoCount + len(repos)
		for _, r := range repos {
			enabled := config.IsBotEnabled(ctx, ic, *r.Owner.Login, *r.Name)
			err = RunPolicies(ctx, ic, *r.Owner.Login, *r.Name, enabled)
			if err != nil {
				break
			}
		}
		if err != nil {
			log.Error().
				Err(err).
				Msg("Unexpected error running policies.")
			continue
		}
	}
	ghc.LogCacheSize()
	log.Info().
		Str("area", "bot").
		Int("count", repoCount).
		Msg("EnforceAll complete.")
	return nil
}

// EnforceJob is a reconcilation job that enforces policies on all repos every
// d duration. It runs forever until the context is done.
func EnforceJob(ctx context.Context, ghc *ghclients.GHClients, d time.Duration) error {
	for {
		err := EnforceAll(ctx, ghc)
		if err != nil {
			log.Error().
				Err(err).
				Msg("Unexpected error enforcing policies.")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
}

// RunPolicies enforces policies on the provided repo. It is meant to be called
// from either jobs, webhooks, or delayed checks. TODO: implement concurrency
// check to only run a single instance per repo at a time.
func RunPolicies(ctx context.Context, c *github.Client, owner, repo string, enabled bool) error {
	ps := policiesGetPolicies()
	for _, p := range ps {
		r, err := p.Check(ctx, c, owner, repo)
		if err != nil {
			return err
		}
		log.Info().
			Str("org", owner).
			Str("repo", repo).
			Str("area", p.Name()).
			Bool("result", r.Pass).
			Bool("enabled", enabled && r.Enabled).
			Str("notify", r.NotifyText).
			Interface("details", r.Details).
			Msg("Policy run result.")
		if !enabled || !r.Enabled {
			continue
		}
		a := p.GetAction(ctx, c, owner, repo)
		if !r.Pass {
			switch a {
			case "log":
			case "issue":
				err := issueEnsure(ctx, c, owner, repo, p.Name(), r.NotifyText)
				if err != nil {
					return err
				}
			case "email":
				log.Warn().
					Str("org", owner).
					Str("repo", repo).
					Str("area", p.Name()).
					Msg("Email action configured, but not implemented yet.")
			case "fix":
				err := p.Fix(ctx, c, owner, repo)
				if err != nil {
					return err
				}
			default:
				log.Warn().
					Str("org", owner).
					Str("repo", repo).
					Str("area", p.Name()).
					Str("action", a).
					Msg("Unknown action configured.")
			}
		}
		if r.Pass && a == "issue" {
			err := issueClose(ctx, c, owner, repo, p.Name())
			if err != nil {
				return err
			}
		}
	}
	return nil
}
