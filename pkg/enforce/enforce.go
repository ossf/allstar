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
	"fmt"
	"sync"
	"time"

	"github.com/contentful/allstar/pkg/config"
	"github.com/contentful/allstar/pkg/config/operator"
	"github.com/contentful/allstar/pkg/ghclients"
	"github.com/contentful/allstar/pkg/issue"
	"github.com/contentful/allstar/pkg/policies"
	"github.com/contentful/allstar/pkg/policydef"
	"github.com/contentful/allstar/pkg/scorecard"
	"golang.org/x/sync/errgroup"

	"github.com/google/go-github/v50/github"
	"github.com/rs/zerolog/log"
)

type EnforceRepoResults = map[string]bool
type EnforceAllResults = map[string]map[string]int

var doNothingOnOptOut = operator.DoNothingOnOptOut
var policiesGetPolicies func() []policydef.Policy
var issueEnsure func(context.Context, *github.Client, string, string, string, string) error
var issueClose func(context.Context, *github.Client, string, string, string) error
var configIsBotEnabled func(context.Context, *github.Client, string, string) bool
var getAppInstallations func(context.Context, *github.Client) ([]*github.Installation, error)
var getAppInstallationRepos func(context.Context, *github.Client) ([]*github.Repository, *github.Response, error)
var runPolicies func(context.Context, *github.Client, string, string, bool, string) (EnforceRepoResults, error)
var deleteInstallation func(context.Context, *github.Client, int64) (*github.Response, error)
var listInstallations func(context.Context, *github.Client) ([]*github.Installation, error)

func init() {
	policiesGetPolicies = policies.GetPolicies
	issueEnsure = issue.Ensure
	issueClose = issue.Close
	configIsBotEnabled = config.IsBotEnabled
	getAppInstallations = getAppInstallationsReal
	getAppInstallationRepos = getAppInstallationReposReal
	runPolicies = runPoliciesReal
	deleteInstallation = deleteInstallationReal
	listInstallations = listInstallationsReal
}

// EnforceAll iterates through all available installations and repos Allstar
// has access to and runs policies on those repos. It is meant to be a
// reconciliation job to check repos which a webhook event may have been lost.
//
// TBD: determine if this should remain exported, or if it will only be called
// from EnforceJob.
func EnforceAll(ctx context.Context, ghc ghclients.GhClientsInterface, specificPolicyArg string, specificRepoArg string) (EnforceAllResults, error) {
	var repoCount int
	var enforceAllResults = make(EnforceAllResults)
	ac, err := ghc.Get(0)
	if err != nil {
		return nil, err
	}
	insts, err := getAppInstallations(ctx, ac)
	if err != nil {
		return nil, err
	}

	log.Info().
		Str("area", "bot").
		Int("count", len(insts)).
		Msg("Enforcing policies on installations.")

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(5)
	var mu sync.Mutex

	for _, i := range insts {
		if i.SuspendedAt != nil {
			log.Info().
				Str("area", "bot").
				Int64("instId", i.GetID()).
				Str("instTarget", i.GetAccount().GetLogin()).
				Msg("Installation is suspended, skipping.")
			continue
		}
		ic, err := ghc.Get(i.GetID())
		if err != nil {
			log.Error().
				Err(err).
				Int64("instId", i.GetID()).
				Str("instTarget", i.GetAccount().GetLogin()).
				Msg("Unexpected error getting installation client.")
			return nil, err
		}
		iid := i.GetID()

		g.Go(func() error {

			repos, _, err := getAppInstallationRepos(ctx, ic)

			if specificRepoArg != "" {
				searchRepos := repos
				repos = make([]*github.Repository, 0, 1)
				for _, r := range searchRepos {
					if r.GetFullName() == specificRepoArg {
						repos = append(repos, r)
						break
					}
				}
			}

			// FIXME, not getting a rsp for this one, instead I think it is a special
			// error that I need to introspect. just continue on all errors here
			// temporarily to fix prod.
			// if err != nil && rsp != nil && rsp.StatusCode == http.StatusForbidden {
			// 	log.Error().
			// 		Err(err).
			// 		Msg("Skip installation, forbidden.")
			// 	continue
			// }
			if err != nil {
				log.Error().
					Err(err).
					Msg("Unexpected error listing installation repos.")
				// return nil, err
				return nil
			}

			log.Info().
				Str("area", "bot").
				Int64("id", iid).
				Int("count", len(repos)).
				Msg("Enforcing policies on repos of installation.")

			instResults, err := runPoliciesOnInstRepos(ctx, repos, ic, specificPolicyArg)

			mu.Lock()
			repoCount = repoCount + len(repos)
			for policyName, results := range instResults {
				if enforceAllResults[policyName] == nil {
					enforceAllResults[policyName] = make(map[string]int)
				}
				enforceAllResults[policyName]["totalFailed"] += results["totalFailed"]
			}
			mu.Unlock()

			if err != nil {
				log.Error().
					Err(err).
					Msg("Unexpected error running policies.")
				return nil
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return enforceAllResults, err
	}
	ghc.LogCacheSize()
	log.Info().
		Str("area", "bot").
		Int("count", repoCount).
		Interface("results", enforceAllResults).
		Msg("EnforceAll complete.")
	return enforceAllResults, nil
}

func runPoliciesOnInstRepos(ctx context.Context, repos []*github.Repository, ghclient *github.Client, specificPolicyArg string) (
	EnforceAllResults, error) {
	var instResults = make(EnforceAllResults)
	var repoLoopErr error
	var owner string
	for _, r := range repos {
		enabled := configIsBotEnabled(ctx, ghclient, *r.Owner.Login, *r.Name)
		enforceResults, err := runPolicies(ctx, ghclient, *r.Owner.Login, *r.Name, enabled, specificPolicyArg)
		if err != nil {
			// scope of err doesn't extend outside the for loop
			repoLoopErr = err
			break
		}
		if owner == "" {
			owner = *r.Owner.Login
		}
		for policyName, passed := range enforceResults {
			if !passed {
				if instResults[policyName] == nil {
					instResults[policyName] = make(map[string]int)
				}
				instResults[policyName]["totalFailed"] += 1
			}
		}
	}
	config.ClearInstLoc(owner)
	return instResults, repoLoopErr
}

func listInstallationsReal(ctx context.Context, ac *github.Client) ([]*github.Installation, error) {
	var insts []*github.Installation
	opts := &github.ListOptions{
		PerPage: 100,
	}
	var err error
	for {
		is, resp, err := ac.Apps.ListInstallations(ctx, opts)

		if err != nil {
			return nil, err
		}

		insts = append(insts, is...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return insts, err
}

func getAppInstallationsReal(ctx context.Context, ac *github.Client) ([]*github.Installation, error) {
	is, err := listInstallations(ctx, ac)
	if err != nil {
		return nil, err
	}

	if len(operator.AllowedOrganizations) == 0 ||
		(len(operator.AllowedOrganizations) == 1 && operator.AllowedOrganizations[0] == "") {
		return is, nil
	}

	var insts []*github.Installation
	for _, i := range is {
		var m bool
		for _, ao := range operator.AllowedOrganizations {
			// Account.Login holds the name of the GitHub Organization
			// that Allstar is installed on
			if i.Account != nil && ao == i.Account.GetLogin() {
				m = true
			}
		}
		if !m {
			resp, err := deleteInstallation(ctx, ac, i.GetID())
			if err != nil || resp.StatusCode != 200 {
				log.Error().
					Err(err).
					Str("area", "bot").
					Str("account", i.Account.GetLogin()).
					Int64("id", i.GetID()).
					Msg("couldn't disable allstar on disallowed repo")
			}

			continue
		}
		insts = append(insts, i)
	}

	return insts, nil
}

func deleteInstallationReal(ctx context.Context, ic *github.Client, instID int64) (*github.Response, error) {
	resp, err := ic.Apps.DeleteInstallation(ctx, instID)
	return resp, err
}

func getAppInstallationReposReal(ctx context.Context, ic *github.Client) ([]*github.Repository, *github.Response, error) {
	var repos []*github.Repository
	opt := &github.ListOptions{
		PerPage: 100,
	}
	var err error
	var resp *github.Response
	for {
		var rs *github.ListRepositories
		rs, resp, err = ic.Apps.ListRepos(ctx, opt)
		if err != nil {
			break
		}
		for _, repo := range rs.Repositories {
			if !repo.GetArchived() {
				repos = append(repos, repo)
				continue
			}
			continue
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return repos, resp, err
}

// EnforceJob is a reconciliation job that enforces policies on all repos every
// d duration. It runs forever until the context is done.
func EnforceJob(ctx context.Context, ghc *ghclients.GHClients, d time.Duration, specificPolicyArg string, specificRepoArg string) error {
	for {
		_, err := EnforceAll(ctx, ghc, specificPolicyArg, specificRepoArg)
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

// runPoliciesReal enforces policies on the provided repo. It is meant to be called
// from either jobs, webhooks, or delayed checks. TODO: implement concurrency
// check to only run a single instance per repo at a time.
func runPoliciesReal(ctx context.Context, c *github.Client, owner, repo string, enabled bool, specificPolicyArg string) (EnforceRepoResults, error) {
	var enforceResults = make(EnforceRepoResults)
	ps := policiesGetPolicies()
	if specificPolicyArg != "" {
		var found policydef.Policy
		for _, p := range ps {
			var policyName = p.Name()
			if policyName == specificPolicyArg {
				found = p
			}
		}
		ps = make([]policydef.Policy, 1)
		ps[0] = found
	}

	defer scorecard.Close(fmt.Sprintf("%s/%s", owner, repo))
	for _, p := range ps {
		repo_enabled, err := p.IsEnabled(ctx, c, owner, repo)
		if err != nil {
			return nil, err
		}
		if !(repo_enabled && enabled) && doNothingOnOptOut {
			log.Info().
				Str("org", owner).
				Str("repo", repo).
				Str("area", p.Name()).
				Msg("Policy run skipped as repo is not enabled and doNothingOnOptOut is configured.")
			continue
		}
		r, err := p.Check(ctx, c, owner, repo)
		if err != nil {
			return nil, err
		}
		log.Info().
			Str("org", owner).
			Str("repo", repo).
			Str("area", p.Name()).
			Bool("result", r.Pass).
			Bool("enabled", r.Enabled).
			Str("notify", r.NotifyText).
			Interface("details", r.Details).
			Msg("Policy run result.")
		if !r.Enabled {
			continue
		}
		a := p.GetAction(ctx, c, owner, repo)
		enforceResults[p.Name()] = r.Pass
		if !r.Pass {
			switch a {
			case "log":
			case "issue":
				err := issueEnsure(ctx, c, owner, repo, p.Name(), r.NotifyText)
				if err != nil {
					return nil, err
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
					return nil, err
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
		if r.Pass && (a == "issue" || a == "fix") {
			err := issueClose(ctx, c, owner, repo, p.Name())
			if err != nil {
				return nil, err
			}
		}
	}

	return enforceResults, nil
}
