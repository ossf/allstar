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

	"github.com/ossf/scorecard/v4/clients"
	"github.com/ossf/scorecard/v4/clients/githubrepo"
)

// Type ScClient is returned from Get. It contains the clients needed to call
// scorecard checks.
type ScClient struct {
	ScRepo       clients.Repo
	ScRepoClient clients.RepoClient
}

var scClients map[string]*ScClient

const defaultGitRef = "HEAD"

var githubrepoMakeGithubRepo func(string) (clients.Repo, error)
var githubrepoCreateGithubRepoClientWithTransport func(context.Context, http.RoundTripper) clients.RepoClient

func init() {
	githubrepoMakeGithubRepo = githubrepo.MakeGithubRepo
	githubrepoCreateGithubRepoClientWithTransport = githubrepo.CreateGithubRepoClientWithTransport
}

// Function Get will get the scorecard clients and create them if they don't
// exist. The github repo is initialized, which means the tarball is
// downloaded.
func Get(ctx context.Context, fullRepo string, tr http.RoundTripper) (*ScClient, error) {
	if scClients == nil {
		scClients = make(map[string]*ScClient)
	}
	if scc, ok := scClients[fullRepo]; ok {
		return scc, nil
	}
	scc, err := create(ctx, fullRepo, tr)
	if err != nil {
		return nil, err
	}
	scClients[fullRepo] = scc
	return scc, nil
}

// Function Close will close the scorecard clients. This cleans up the
// downloaded tarball.
func Close(fullRepo string) {
	if scClients == nil {
		scClients = make(map[string]*ScClient)
	}
	if _, ok := scClients[fullRepo]; !ok {
		return
	}
	scClients[fullRepo].ScRepoClient.Close()
	delete(scClients, fullRepo)
}

func create(ctx context.Context, fullRepo string, tr http.RoundTripper) (*ScClient, error) {
	scr, err := githubrepoMakeGithubRepo(fullRepo)
	if err != nil {
		return nil, err
	}
	scrc := githubrepoCreateGithubRepoClientWithTransport(ctx, tr)
	if err := scrc.InitRepo(scr, defaultGitRef); err != nil {
		return nil, err
	}
	return &ScClient{
		ScRepo:       scr,
		ScRepoClient: scrc,
	}, nil
}
