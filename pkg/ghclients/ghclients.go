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

// Package ghclients stores ghclients with caching and auth for installations
// of a GitHub App
package ghclients

import (
	"net/http"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v35/github"
	"github.com/gregjones/httpcache"
)

const config_AppID = 1
const config_KeyFile = "key.pem"

var ghinstallationNewAppsTransportKeyFromFile func(http.RoundTripper, int64,
	string) (*ghinstallation.AppsTransport, error)
var ghinstallationNewKeyFromFile func(http.RoundTripper, int64, int64, string) (
	*ghinstallation.Transport, error)

func init() {
	ghinstallationNewAppsTransportKeyFromFile = ghinstallation.NewAppsTransportKeyFromFile
	ghinstallationNewKeyFromFile = ghinstallation.NewKeyFromFile
}

// GHClients stores clients per-installation for re-use througout a process.
type GHClients struct {
	clients map[int64]*github.Client
	tr      http.RoundTripper
}

// NewGHClients returns a new GHClients. The provided RoundTripper will be
// stored and used when creating new clients.
func NewGHClients(t http.RoundTripper) *GHClients {
	return &GHClients{
		clients: make(map[int64]*github.Client),
		tr:      t,
	}
}

// Get gets the client for installation id i, If i is 0 it gets the client for
// the app-level api. If a stored client is not available, it creates a new
// client with auth and caching built in.
func (g *GHClients) Get(i int64) (*github.Client, error) {
	if c, ok := g.clients[i]; ok {
		return c, nil
	}
	var tr http.RoundTripper
	var err error
	if i == 0 {
		tr, err = ghinstallationNewAppsTransportKeyFromFile(g.tr, config_AppID, config_KeyFile)
	} else {
		tr, err = ghinstallationNewKeyFromFile(g.tr, config_AppID, i, config_KeyFile)
	}
	if err != nil {
		return nil, err
	}
	ctr := &httpcache.Transport{
		Transport:           tr,
		Cache:               httpcache.NewMemoryCache(),
		MarkCachedResponses: true,
	}
	g.clients[i] = github.NewClient(&http.Client{Transport: ctr})
	return g.clients[i], nil
}
