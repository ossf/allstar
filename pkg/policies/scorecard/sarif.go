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
	"sync"

	"github.com/google/go-github/v74/github"
	"github.com/rs/zerolog/log"

	"github.com/ossf/scorecard/v5/clients"
	docs "github.com/ossf/scorecard/v5/docs/checks"
	sclog "github.com/ossf/scorecard/v5/log"
	"github.com/ossf/scorecard/v5/options"
	sc "github.com/ossf/scorecard/v5/pkg/scorecard"
	spol "github.com/ossf/scorecard/v5/policy"
)

// Mockable function variables for testing.
var (
	codeScanningUploadFunc  = codeScanningUploadReal
	getDefaultBranchRefFunc = getDefaultBranchRefReal
)

// Change detection: in-memory hash of last uploaded SARIF per repo.
var (
	sarifHashMu  sync.Mutex
	sarifHashMap = make(map[string]string) // "owner/repo" -> SHA-256 hex
)

// generateSARIF runs all configured checks at once and writes the results
// as SARIF to the provided writer.
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

	return resultToSARIF(&result, checkNames, threshold, writer)
}

// resultToSARIF converts a scorecard Result to SARIF and writes it
// to the provided writer.
func resultToSARIF(
	result *sc.Result,
	checkNames []string,
	threshold int,
	writer io.Writer,
) error {
	policy := buildPolicy(checkNames, threshold)

	checkDocs, err := docs.Read()
	if err != nil {
		return fmt.Errorf("reading check docs: %w", err)
	}

	opts := &options.Options{}

	return result.AsSARIF(true, sclog.DefaultLevel, writer, checkDocs, policy, opts)
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

	return uploadToCodeScanningWithRef(ctx, c, owner, repo, ref, sha, sarifContent)
}

// uploadToCodeScanningWithRef uploads SARIF content to GitHub's Code Scanning
// API using a pre-fetched ref and commit SHA.
func uploadToCodeScanningWithRef(
	ctx context.Context,
	c *github.Client,
	owner, repo string,
	ref, commitSHA string,
	sarifContent []byte,
) error {
	encoded, err := compressAndEncode(sarifContent)
	if err != nil {
		return fmt.Errorf("compressing SARIF: %w", err)
	}

	analysis := &github.SarifAnalysis{
		CommitSHA: github.Ptr(commitSHA),
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

// uploadSARIF generates SARIF for the configured checks and uploads
// it to GitHub's Code Scanning API, skipping the upload if the SARIF content
// has not changed since the last upload.
func uploadSARIF(
	ctx context.Context,
	c *github.Client,
	owner, repo string,
	checkNames []string,
	threshold int,
	scRepo clients.Repo,
	scRepoClient clients.RepoClient,
) error {
	// Change detection: skip upload if the repo HEAD hasn't changed
	// since the last upload. This avoids redundant uploads when Allstar
	// runs on a 5-minute cycle and the repo hasn't been updated.
	ref, commitSHA, err := getDefaultBranchRefFunc(ctx, c, owner, repo)
	if err != nil {
		return fmt.Errorf("getting branch info: %w", err)
	}

	repoKey := fmt.Sprintf("%s/%s", owner, repo)

	sarifHashMu.Lock()
	lastSHA := sarifHashMap[repoKey]
	sarifHashMu.Unlock()

	if commitSHA == lastSHA {
		log.Debug().
			Str("org", owner).
			Str("repo", repo).
			Str("area", polName).
			Msg("SARIF unchanged, skipping upload.")
		return nil
	}

	// Run all checks at once to get a complete Result.
	runOpts := []sc.Option{
		sc.WithChecks(checkNames),
	}
	if scRepoClient != nil {
		runOpts = append(runOpts, sc.WithRepoClient(scRepoClient))
	}

	result, err := scRun(ctx, scRepo, runOpts...)
	if err != nil {
		return fmt.Errorf("scorecard run for SARIF: %w", err)
	}

	// Generate SARIF from the result.
	var buf bytes.Buffer
	if err := resultToSARIF(&result, checkNames, threshold, &buf); err != nil {
		return fmt.Errorf("generating SARIF: %w", err)
	}

	if err := uploadToCodeScanningWithRef(ctx, c, owner, repo, ref, commitSHA, buf.Bytes()); err != nil {
		return err
	}

	sarifHashMu.Lock()
	sarifHashMap[repoKey] = commitSHA
	sarifHashMu.Unlock()

	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Msg("SARIF uploaded to Code Scanning.")

	return nil
}


// clearSARIFHashes resets the change detection state. Exported for testing.
func clearSARIFHashes() {
	sarifHashMu.Lock()
	sarifHashMap = make(map[string]string)
	sarifHashMu.Unlock()
}
