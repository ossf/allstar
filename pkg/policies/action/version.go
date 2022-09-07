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
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-github/v43/github"
)

// comparisonRegex matches comparisons
// eg. ">= a50145de5a27a8353fc755392d08b1f7ad5a30e8",
// "a50145de5a27a8353fc755392d08b1f7ad5a30e8", "< main", "=main"
// Group 1 is comparator if any (none implies =), group 2 should be versionish
var comparisonRegex = regexp.MustCompile(`^((?:>=)|>|<|(?:<=)|=)? *([^=>< ]+)$`)

type versionishKind int

const (
	versionishKindCommitID versionishKind = iota
	versionishKindBranchRef
	versionishKindTagRef
)

type version struct {
	commitID string
}

func newVersion(commitID string) *version {
	return &version{commitID: commitID}
}

// Attempts to create a new *version from a versionish value (could be a commit
// hash, branch, tag)
// Returns the versionishKind that applies to versionish. Do not use on error.
func newVersionFromVersionish(ctx context.Context, c *github.Client, owner, repo, versionish string) (*version, versionishKind, error) {
	errPrefix := fmt.Sprintf("while resolving version for versionish \"%s\" in repo \"%s/%s\"", versionish, owner, repo)
	// Did not check if versionish matches commitHashRegex
	// regexp.MustCompile(`^[0-9a-z]{40}$`) here because we should use
	// listCommits to check if versionish matches commit.
	commits, err := listCommits(ctx, c, owner, repo)
	if err != nil {
		return nil, -1, fmt.Errorf("%s: %w", errPrefix, err)
	}
	for _, c := range commits {
		if c.GetSHA() == versionish {
			return newVersion(c.GetSHA()), versionishKindCommitID, nil
		}
	}
	// If not commit, attempt locate tag (we can now assume m.version is a commit ref of some sort)
	tags, err := listTags(ctx, c, owner, repo)
	if err != nil {
		return nil, -1, fmt.Errorf("%s: %w", errPrefix, err)
	}
	for _, tag := range tags {
		commitSHA := tag.GetCommit().GetSHA()
		if tag.GetName() == versionish && commitSHA != "" {
			return &version{commitID: commitSHA}, versionishKindTagRef, nil
		}
	}
	// ...or maybe it is a branch?
	branches, err := listBranches(ctx, c, owner, repo)
	if err != nil {
		return nil, -1, fmt.Errorf("%s: %w", errPrefix, err)
	}
	for _, b := range branches {
		commitSHA := b.GetCommit().GetSHA()
		if b.GetName() == versionish && commitSHA != "" {
			return &version{commitID: commitSHA}, versionishKindBranchRef, nil
		}
	}
	// not a commit, tag, or branch
	return nil, -1, fmt.Errorf("%s: no corresponding commit found for \"%s\"", errPrefix, versionish)
}

func (v *version) Equals(o *version) bool {
	return v.commitID == o.commitID
}

func (v *version) CommitID() string {
	return v.commitID
}

type versionConstraint struct {
	comp    comparison
	version *version
	vkind   versionishKind
}

type comparison string

const (
	gte comparison = ">="
	gt  comparison = ">"
	lte comparison = "<="
	lt  comparison = "<"
	eq  comparison = "="
)

var comparisons = []comparison{gte, gt, lte, lt, eq}

func toComparison(s string) (comparison, error) {
	for _, c := range comparisons {
		if string(c) == s {
			return c, nil
		}
	}
	return "", fmt.Errorf("invalid comparison operator \"%s\"", s)
}

func parseVersionConstraint(ctx context.Context, c *github.Client, owner, repo, versionish string) (*versionConstraint, error) {
	errPrefix := fmt.Sprintf("while parsing version constraint \"%s\"", versionish)
	rg := comparisonRegex.FindStringSubmatch(versionish)
	if rg == nil || len(rg) < 3 {
		return nil, fmt.Errorf("%s: invalid constraint", errPrefix)
	}
	if rg[1] == "" {
		rg[1] = "="
	}
	comp, err := toComparison(rg[1])
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	version, vkind, err := newVersionFromVersionish(ctx, c, owner, repo, rg[2])
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	return &versionConstraint{comp, version, vkind}, nil
}

func (vc *versionConstraint) Match(ctx context.Context, c *github.Client, owner, repo string, v *version) (bool, error) {
	errPrefix := "while matching version constraint"
	if vc.comp == eq {
		return vc.version.Equals(v), nil
	}
	commits, err := listCommits(ctx, c, owner, repo)
	if err != nil {
		return false, fmt.Errorf("%s: %v", errPrefix, err)
	}
	successorsPredecessors := map[string][]string{}
	if vc.comp == gt || vc.comp == gte {
		// find successors
		for _, c := range commits {
			cid := c.GetSHA()
			for _, p := range c.Parents {
				pid := p.GetSHA()
				if _, ok := successorsPredecessors[pid]; !ok {
					successorsPredecessors[pid] = []string{}
				}
				successorsPredecessors[pid] = append(successorsPredecessors[pid], cid)
			}
		}
	} else {
		// find predecessors
		for _, c := range commits {
			cid := c.GetSHA()
			successorsPredecessors[cid] = []string{}
			for _, p := range c.Parents {
				successorsPredecessors[cid] = append(successorsPredecessors[cid], p.GetSHA())
			}
		}
	}
	// bfs on successorsPredecessors to figure out if v is in right direction
	visited := map[string]struct{}{}
	queue := []string{vc.version.CommitID()}
	// => skip vc.version if equal does not qualify
	if vc.comp == gt || vc.comp == lt {
		queue = successorsPredecessors[vc.version.CommitID()]
		visited[vc.version.CommitID()] = struct{}{}
	}
	for len(queue) > 0 {
		current := queue[0]
		if _, ok := visited[current]; ok {
			// Already visited this commit
			queue = queue[1:]
			continue
		}
		queue = append(queue, successorsPredecessors[current]...)
		visited[current] = struct{}{}
		queue = queue[1:]

		if current == v.CommitID() {
			// found in graph (correct direction)

			// if vkind (original parsed version kind) is tag, ensure this
			// commit has a tag
			if vc.vkind == versionishKindTagRef {
				tags, err := listTags(ctx, c, owner, repo)
				if err != nil {
					return false, err
				}
				hasTag := false
				for _, t := range tags {
					if t.GetCommit().GetSHA() == current {
						hasTag = true
					}
				}
				if !hasTag {
					// No tag for matching commit
					continue
				}
			}

			return true, nil
		}
	}
	// not found in graph
	return false, nil
}
