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

// Package scorecard handles sharing a Scorecard githubrepo client across
// multiple Allstar policies. The Allstar policy interface does not handle
// passing a reference to this, so we will store a master state here and look
// it up each time. We don't want to keep the tarball around forever, or we
// will run out of disk space. This is intended to be setup once for a repo,
// then all policies run, then closed for that repo.
package scorecard

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	plumbinghttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v59/github"
	"github.com/ossf/allstar/pkg/ghclients"
	"github.com/ossf/scorecard/v5/clients"
	"github.com/ossf/scorecard/v5/clients/githubrepo"
	"github.com/ossf/scorecard/v5/clients/localdir"
	"github.com/rs/zerolog/log"
)

// Type ScClient is returned from Get. It contains the clients needed to call
// scorecard checks.
type ScClient struct {
	ScRepo       clients.Repo
	ScRepoClient clients.RepoClient
	localPath    string
	gitRepo      *git.Repository
}

var scClientsRemote map[string]*ScClient = make(map[string]*ScClient)
var scClientsLocal map[string]*ScClient = make(map[string]*ScClient)
var mMutex sync.RWMutex

const defaultGitRef = "HEAD"

var githubrepoMakeGitHubRepo func(string) (clients.Repo, error)
var githubrepoCreateGitHubRepoClientWithTransport func(context.Context, http.RoundTripper) clients.RepoClient
var createLocal func(context.Context, string) (*ScClient, error)

func init() {
	githubrepoMakeGitHubRepo = githubrepo.MakeGithubRepo
	githubrepoCreateGitHubRepoClientWithTransport = githubrepo.CreateGithubRepoClientWithTransport
	createLocal = _createLocal
}

// Function Get will get the scorecard clients and create them if they don't
// exist. The github repo is initialized, which means the tarball is
// downloaded.
// If local is set, a local copy of the repo will be used instead, this allows for branch
// changes without another scorecard repo download.
func Get(ctx context.Context, fullRepo string, local bool, tr http.RoundTripper) (*ScClient, error) {
	defer mMutex.Unlock()
	mMutex.Lock()
	if local {
		if scc, ok := scClientsLocal[fullRepo]; ok {
			return scc, nil
		}
		scc, err := createLocal(ctx, fullRepo)
		if err != nil {
			return nil, err
		}
		scClientsLocal[fullRepo] = scc
		return scc, nil

	} else {
		// remote
		if scc, ok := scClientsRemote[fullRepo]; ok {
			return scc, nil
		}
		scc, err := createRemote(ctx, fullRepo, tr)
		if err != nil {
			return nil, err
		}
		scClientsRemote[fullRepo] = scc
		return scc, nil
	}
}

// Switch the local repo between branches
func (scc ScClient) SwitchLocalBranch(branchName string) error {
	log.Debug().
		Str("branch", branchName).
		Msg("Branch checkout")

	w, err := scc.gitRepo.Worktree()
	if err != nil {
		return err
	}
	err = w.Checkout(&git.CheckoutOptions{Branch: plumbing.ReferenceName(branchName)})
	if err != nil {
		return err
	}
	return nil
}

// Fetch branches from the local repo (used for workflow rule checking)
func (scc ScClient) FetchBranches() ([]string, error) {
	refs, err := scc.gitRepo.References()
	if err != nil {
		return nil, err
	}

	var ret []string
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() {
			ret = append(ret, ref.Name().String())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Checkout a repo into a local directory
// returns the path to the local repo and a git repo reference
func checkoutRepo(fullRepo string, token string) (string, *git.Repository, error) {
	log.Debug().
		Str("repo", fullRepo).
		Msg("Repo checkout")
	// create a temp dir to store the repo
	dir, err := os.MkdirTemp("", "allstar")
	if err != nil {
		return "", nil, err
	}

	// can be empty for testing
	var auth *plumbinghttp.BasicAuth
	if token != "" {
		auth = &plumbinghttp.BasicAuth{
			Username: "x-access-token", // can be any value
			Password: token,
		}
	}

	// checkout to the temp dir
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:  "https://github.com/" + fullRepo,
		Auth: auth,
	})
	if err != nil {
		return "", nil, err
	}
	return dir, repo, nil
}

// Function Close will close the scorecard clients. This cleans up the
// downloaded tarball.
func Close(fullRepo string) {
	defer mMutex.Unlock()
	mMutex.Lock()
	// local
	scc, ok := scClientsLocal[fullRepo]
	if ok {
		// delete local temp directory
		os.RemoveAll(scc.localPath)
		delete(scClientsLocal, fullRepo)
	}
	// remote
	scc, ok = scClientsRemote[fullRepo]
	if ok {
		scc.ScRepoClient.Close()
		delete(scClientsRemote, fullRepo)
	}
}

// As we are potentially running against a private GitHub repo, we need to use a two step process
// to checkout a local copy of the full repo:
// 1) Use our go-github API access to create a scoped installation access token
// 2) Use the access token to clone the repo with go-git
// ref: https://github.com/orgs/community/discussions/24575#discussioncomment-8076322
func _createLocal(ctx context.Context, fullRepo string) (*ScClient, error) {
	fullRepoSplit := strings.Split(fullRepo, "/")
	owner := strings.ToLower(fullRepoSplit[0])
	repo := strings.ToLower(fullRepoSplit[1])

	// connect to the GitHub API
	ghc, err := ghclients.NewGHClients(ctx, http.DefaultTransport)
	if err != nil {
		return nil, err
	}
	client, err := ghc.Get(0)
	if err != nil {
		return nil, err
	}

	log.Debug().
		Str("owner", owner).
		Str("repo", repo).
		Str("fullRepo", fullRepo).
		Msg("local checkout")

	// use the GitHub API to issue an installation access token
	installation, _, err := client.Apps.FindRepositoryInstallation(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	log.Debug().
		Int64("ID", installation.GetID()).
		Msg("local checkout -- found installation ID")
	opts := &github.InstallationTokenOptions{
		Repositories: []string{repo},
		Permissions: &github.InstallationPermissions{
			Contents: github.String("read"),
		},
	}
	token, _, err := client.Apps.CreateInstallationToken(ctx, installation.GetID(), opts)
	if err != nil {
		return nil, err
	}
	log.Debug().
		Msg("local checkout -- installation access token retrieved")

	// use the installation access token
	localPath, gitRepo, err := checkoutRepo(fullRepo, token.GetToken())
	if err != nil {
		return nil, err
	}
	scr, err := localdir.MakeLocalDirRepo(localPath)
	if err != nil {
		return nil, err
	}
	log.Debug().
		Msg("local checkout -- checkout success")

	return &ScClient{
		ScRepo:    scr,
		localPath: localPath,
		gitRepo:   gitRepo,
	}, nil
}

func createRemote(ctx context.Context, fullRepo string, tr http.RoundTripper) (*ScClient, error) {
	scr, err := githubrepoMakeGitHubRepo(fullRepo)
	if err != nil {
		return nil, err
	}
	scrc := githubrepoCreateGitHubRepoClientWithTransport(ctx, tr)
	if err := scrc.InitRepo(scr, defaultGitRef, 0); err != nil {
		return nil, err
	}
	return &ScClient{
		ScRepo:       scr,
		ScRepoClient: scrc,
	}, nil
}
