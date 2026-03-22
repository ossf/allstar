// Copyright 2026 OpenSSF Scorecard Authors

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scorecard

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/google/go-github/v74/github"

	"github.com/ossf/scorecard/v5/clients"
	docs "github.com/ossf/scorecard/v5/docs/checks"
	"github.com/ossf/scorecard/v5/log"
	"github.com/ossf/scorecard/v5/options"
	sc "github.com/ossf/scorecard/v5/pkg/scorecard"
	spol "github.com/ossf/scorecard/v5/policy"
)

// Mockable function variables for testing.
var (
	codeScanningUploadFunc = codeScanningUploadReal
	getDefaultBranchRefFunc = getDefaultBranchRefReal
)

// generateSARIF runs all configured checks at once and writes the results
// as SARIF 2.1.0 to the provided writer.
//
// This is the "dual execution" path: the main Check() method runs checks
// individually for issue text, while this function runs them together to
// produce a complete SARIF document.
func generateSARIF(
	ctx context.Context,
	repo clients.Repo,
	repoClient clients.RepoClient,
	checkNames []string,
	threshold int,
	writer io.Writer,
) error {
	// Run all checks at once to get a complete Result.
	runOpts := []sc.Option{
		sc.WithChecks(checkNames),
	}
	if repoClient != nil {
		runOpts = append(runOpts, sc.WithRepoClient(repoClient))
	}

	result, err := scRun(ctx, repo, runOpts...)
	if err != nil {
		return fmt.Errorf("scorecard run for SARIF: %w", err)
	}

	// Build a ScorecardPolicy from Allstar's checks + threshold config.
	policy := buildPolicy(checkNames, threshold)

	// Get check documentation for SARIF remediation guidance.
	checkDocs, err := docs.Read()
	if err != nil {
		return fmt.Errorf("reading check docs: %w", err)
	}

	opts := &options.Options{}

	return result.AsSARIF(true, log.DefaultLevel, writer, checkDocs, policy, opts)
}

// buildPolicy constructs a ScorecardPolicy from Allstar's check names and
// threshold. Each configured check is marked as enforced with the threshold
// as its minimum score.
func buildPolicy(checkNames []string, threshold int) *spol.ScorecardPolicy {
	policy := &spol.ScorecardPolicy{
		Version:  1,
		Policies: make(map[string]*spol.CheckPolicy, len(checkNames)),
	}
	for _, name := range checkNames {
		policy.Policies[name] = &spol.CheckPolicy{
			Score: int32(threshold),
			Mode:  spol.CheckPolicy_ENFORCED,
		}
	}
	return policy
}

// uploadToCodeScanning uploads SARIF content to GitHub's Code Scanning API
// for the given repository.
func uploadToCodeScanning(
	ctx context.Context,
	c *github.Client,
	owner, repo string,
	sarifContent []byte,
) error {
	ref, sha, err := getDefaultBranchRefFunc(ctx, c, owner, repo)
	if err != nil {
		return fmt.Errorf("getting branch info: %w", err)
	}

	encoded, err := compressAndEncode(sarifContent)
	if err != nil {
		return fmt.Errorf("compressing SARIF: %w", err)
	}

	analysis := &github.SarifAnalysis{
		CommitSHA: github.Ptr(sha),
		Ref:       github.Ptr(ref),
		Sarif:     github.Ptr(encoded),
		ToolName:  github.Ptr("OpenSSF Scorecard"),
	}

	_, _, err = codeScanningUploadFunc(ctx, c, owner, repo, analysis)
	if err != nil {
		return fmt.Errorf("uploading SARIF: %w", err)
	}
	return nil
}

func codeScanningUploadReal(
	ctx context.Context,
	c *github.Client,
	owner, repo string,
	analysis *github.SarifAnalysis,
) (*github.SarifID, *github.Response, error) {
	return c.CodeScanning.UploadSarif(ctx, owner, repo, analysis)
}

// getDefaultBranchRefReal gets the default branch ref and HEAD commit SHA
// for a repository.
func getDefaultBranchRefReal(
	ctx context.Context,
	c *github.Client,
	owner, repo string,
) (ref, sha string, err error) {
	r, _, err := c.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return "", "", fmt.Errorf("getting repo: %w", err)
	}
	branch := r.GetDefaultBranch()
	if branch == "" {
		branch = "main"
	}

	b, _, err := c.Repositories.GetBranch(ctx, owner, repo, branch, 0)
	if err != nil {
		return "", "", fmt.Errorf("getting branch %s: %w", branch, err)
	}

	return fmt.Sprintf("refs/heads/%s", branch), b.GetCommit().GetSHA(), nil
}

// compressAndEncode gzip-compresses data and returns it as a base64-encoded
// string. This is the format required by the GitHub Code Scanning API.
func compressAndEncode(data []byte) (string, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)

	if _, err := writer.Write(data); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
