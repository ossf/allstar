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
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

func init() {
	githubrepoMakeGitHubRepo = githubrepo.MakeGithubRepo
	githubrepoCreateGitHubRepoClientWithTransport = githubrepo.CreateGithubRepoClientWithTransport
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
	log.Info().
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
func checkoutRepo(fullRepo string) (string, *git.Repository, error) {
	log.Info().
		Str("repo", fullRepo).
		Msg("Repo checkout")
	// create a temp dir to store the repo
	dir, err := os.MkdirTemp("", "allstar")
	if err != nil {
		return "", nil, err
	}

	// checkout to the temp dir
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL: "https://github.com/" + fullRepo,
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

func createLocal(ctx context.Context, fullRepo string) (*ScClient, error) {
	localPath, gitRepo, err := checkoutRepo(fullRepo)
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
