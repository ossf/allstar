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

//package enforce enforces policies
package enforce

import (
	"context"
	"log"
	"time"

	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/ghclients"
	"github.com/ossf/allstar/pkg/issue"
	"github.com/ossf/allstar/pkg/policies"
	"github.com/ossf/allstar/pkg/policydef"

	"github.com/google/go-github/v35/github"
)

var policiesGetPolicies func() []policydef.Policy
var issueEnsure func(ctx context.Context, c *github.Client, owner, repo, policy, text string) error
var issueClose func(ctx context.Context, c *github.Client, owner, repo, policy string) error

func init() {
	policiesGetPolicies = policies.GetPolicies
	issueEnsure = issue.Ensure
	issueClose = issue.Close
}

func EnforceAll(ctx context.Context, ghc *ghclients.GHClients) error {
	ac, err := ghc.Get(0)
	if err != nil {
		return err
	}
	insts, _, err := ac.Apps.ListInstallations(ctx, nil)
	if err != nil {
		return err
	}
	for _, i := range insts {
		ic, err := ghc.Get(*i.ID)
		if err != nil {
			return err
		}
		repos, _, err := ic.Apps.ListRepos(ctx, nil)
		if err != nil {
			return err
		}
		for _, r := range repos.Repositories {
			if config.IsBotEnabled(ctx, ic, *r.Owner.Login, *r.Name) {
				err := RunPolicies(ctx, ic, *r.Owner.Login, *r.Name)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func EnforceJob(ctx context.Context, ghc *ghclients.GHClients, d time.Duration) error {
	for {
		err := EnforceAll(ctx, ghc)
		if err != nil {
			log.Printf("Error enforcing policies: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
}

func RunPolicies(ctx context.Context, c *github.Client, owner, repo string) error {
	ps := policiesGetPolicies()
	for _, p := range ps {
		r, err := p.Check(ctx, c, owner, repo)
		if err != nil {
			return err
		}
		log.Printf("Policy %v pass: %v", p.Name(), r.Pass)
		log.Printf("Detailed status: %v", r.Details)
		log.Printf("Notify text: %q", r.NotifyText)
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
				log.Print("Email action not implemented yet.")
			case "fix":
				err := p.Fix(ctx, c, owner, repo)
				if err != nil {
					return err
				}
			default:
				log.Printf("Unknown action for policy %v : %v", p.Name(), a)
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
