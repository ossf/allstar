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
	"sync"

	"github.com/ossf/scorecard/v5/clients"
	"github.com/ossf/scorecard/v5/clients/githubrepo"
)

// Type ScClient is returned from Get. It contains the clients needed to call
// scorecard checks.
type ScClient struct {
	ScRepo       clients.Repo
	ScRepoClient clients.RepoClient
}

var scClients map[string]*ScClient = make(map[string]*ScClient)
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
func Get(ctx context.Context, fullRepo string, tr http.RoundTripper) (*ScClient, error) {
	mMutex.Lock()
	if scc, ok := scClients[fullRepo]; ok {
		mMutex.Unlock()
		return scc, nil
	}
	scc, err := create(ctx, fullRepo, tr)
	if err != nil {
		mMutex.Unlock()
		return nil, err
	}
	scClients[fullRepo] = scc
	mMutex.Unlock()
	return scc, nil
}

// Function Close will close the scorecard clients. This cleans up the
// downloaded tarball.
func Close(fullRepo string) {
	mMutex.Lock()
	scc, ok := scClients[fullRepo]
	if !ok {
		mMutex.Unlock()
		return
	}
	scc.ScRepoClient.Close()
	delete(scClients, fullRepo)
	mMutex.Unlock()
}

func create(ctx context.Context, fullRepo string, tr http.RoundTripper) (*ScClient, error) {
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
