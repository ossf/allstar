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

package config

import (
	"context"
	"net/http"

	"github.com/ossf/allstar/pkg/config/operator"
)

type instLoc struct {
	Exists bool
	Repo   string
	Path   string
}

var instLocs map[string]*instLoc

// Function getInstLoc gets the location of org-level configuration for this
// org/installation. The purpose is to only hit possible 404s once per run.
func getInstLoc(ctx context.Context, r repositories, owner string) (*instLoc, error) {
	if instLocs == nil {
		instLocs = make(map[string]*instLoc)
	}
	if il, ok := instLocs[owner]; ok {
		return il, nil
	}
	il, err := createIl(ctx, r, owner)
	if err != nil {
		return nil, err
	}
	instLocs[owner] = il
	return il, nil
}

// Function ClearInstLoc clears any saved config locations for an org/installation
func ClearInstLoc(owner string) {
	if instLocs == nil {
		return
	}
	if _, ok := instLocs[owner]; !ok {
		return
	}
	delete(instLocs, owner)
}

func createIl(ctx context.Context, r repositories, owner string) (*instLoc, error) {
	_, rsp, err := r.Get(ctx, owner, operator.OrgConfigRepo)
	if err == nil {
		// ".allstar" repo exists
		return &instLoc{
			Exists: true,
			Repo:   operator.OrgConfigRepo,
			Path:   "",
		}, nil
	} else if rsp != nil && rsp.StatusCode == http.StatusNotFound {
		// ".allstar" repo does not exist
		_, rsp, err := r.Get(ctx, owner, githubConfRepo)
		if err == nil {
			// ".github" repo exists
			// ".github/allstar" may not exist but we will walk the path on any
			// getcontents to avoid a 404 for that
			return &instLoc{
				Exists: true,
				Repo:   githubConfRepo,
				Path:   operator.OrgConfigDir,
			}, nil
		} else if rsp != nil && rsp.StatusCode == http.StatusNotFound {
			// ".github" repo does not exist
			return &instLoc{
				Exists: false,
			}, nil
		} else {
			// Unknown error getting ".github" repo
			return nil, err
		}
	} else {
		// Unknown error getting ".allstar" repo
		return nil, err
	}
}
