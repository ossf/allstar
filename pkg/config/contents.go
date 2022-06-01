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
	"errors"
	"net/http"
	"path"

	"github.com/google/go-github/v43/github"
)

func walkGetContents(ctx context.Context, r repositories, owner, repo, p string,
	opt *github.RepositoryContentGetOptions) (*github.RepositoryContent,
	[]*github.RepositoryContent, *github.Response, error) {
	paths := makePaths(p)
	for _, v := range paths {
		dir, file := path.Split(v)
		_, rcs, rsp, err := r.GetContents(ctx, owner, repo, dir, opt)
		if rcs == nil || err != nil {
			return nil, nil, rsp, err
		}
		if !fileExists(file, rcs) {
			return nil, nil, &github.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("Not found")
		}
	}
	// File should exist
	return r.GetContents(ctx, owner, repo, p, opt)
}

func makePaths(p string) []string {
	var rv []string
	current := p
	rv = append(rv, current)
	for path.Dir(current) != "." {
		rv = append(rv, path.Dir(current))
		current = path.Dir(current)
	}
	for i, j := 0, len(rv)-1; i < j; i, j = i+1, j-1 {
		rv[i], rv[j] = rv[j], rv[i]
	}
	return rv
}

func fileExists(file string, rcs []*github.RepositoryContent) bool {
	for _, rc := range rcs {
		if file == rc.GetName() {
			return true
		}
	}
	return false
}
