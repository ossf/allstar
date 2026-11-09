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

package action

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/gobwas/glob"
)

// globCache is a cache for compiled globs.
type globCache map[string]glob.Glob

// newGlobCache returns a new globCache.
func newGlobCache() globCache {
	return globCache{}
}

// compileGlob returns cached glob if present, otherwise attempts glob.Compile.
func (g globCache) compileGlob(s string) (glob.Glob, error) {
	if cached, ok := g[s]; ok {
		return cached, nil
	}
	c, err := glob.Compile(s)
	if err != nil {
		return nil, fmt.Errorf("compiling action glob %q: %w", s, err)
	}
	g[s] = c
	return c, nil
}

// semverCache is a cache for compiled versions & constraints
// globCache is a cache for compiled globs.
type semverCache struct {
	version     map[string]*semver.Version
	constraints map[string]*semver.Constraints
}

// newSemverCache returns a new semverCache.
func newSemverCache() semverCache {
	return semverCache{
		version:     map[string]*semver.Version{},
		constraints: map[string]*semver.Constraints{},
	}
}

// compileVersion returns cached Version if present, otherwise attempts
// semver.NewVersion.
func (c semverCache) compileVersion(s string) (*semver.Version, error) {
	if v, ok := c.version[s]; ok {
		return v, nil
	}
	nv, err := semver.NewVersion(s)
	if err != nil {
		return nil, fmt.Errorf("parsing action version %q: %w", s, err)
	}
	c.version[s] = nv
	return nv, nil
}

// compileConstraints recognizes semantic version constraints and caches them.
// Other selectors are literal Git refs, so a parse failure is a normal non-match.
func (c semverCache) compileConstraints(s string) (*semver.Constraints, bool) {
	if v, ok := c.constraints[s]; ok {
		return v, true
	}
	nc, err := semver.NewConstraint(s)
	if err != nil {
		return nil, false
	}
	c.constraints[s] = nc
	return nc, true
}
