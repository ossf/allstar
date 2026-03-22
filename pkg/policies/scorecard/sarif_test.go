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
	"errors"
	"io"
	"testing"

	"github.com/google/go-github/v74/github"

	"github.com/ossf/scorecard/v5/checker"
	"github.com/ossf/scorecard/v5/clients"
	sc "github.com/ossf/scorecard/v5/pkg/scorecard"
)

func TestGenerateSARIF(t *testing.T) {
	tests := []struct {
		Name      string
		Checks    []string
		Threshold int
		Result    sc.Result
		WantErr   bool
	}{
		{
			Name:      "BasicGeneration",
			Checks:    []string{"Binary-Artifacts"},
			Threshold: 8,
			Result: sc.Result{
				Repo: sc.RepoInfo{
					Name: "github.com/test/repo",
				},
				Checks: []checker.CheckResult{
					{
						Name:  "Binary-Artifacts",
						Score: 10,
					},
				},
			},
			WantErr: false,
		},
		{
			Name:      "MultipleChecks",
			Checks:    []string{"Binary-Artifacts", "Signed-Releases"},
			Threshold: 5,
			Result: sc.Result{
				Repo: sc.RepoInfo{
					Name: "github.com/test/repo",
				},
				Checks: []checker.CheckResult{
					{
						Name:  "Binary-Artifacts",
						Score: 10,
					},
					{
						Name:  "Signed-Releases",
						Score: 3,
					},
				},
			},
			WantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Mock scRun to return our test result
			origScRun := scRun
			t.Cleanup(func() { scRun = origScRun })
			scRun = func(_ context.Context, _ clients.Repo, _ ...sc.Option) (sc.Result, error) {
				return test.Result, nil
			}

			var buf bytes.Buffer
			err := generateSARIF(
				context.Background(),
				nil, // ScClient.ScRepo not needed with mocked scRun
				nil, // ScClient.ScRepoClient not needed with mocked scRun
				test.Checks,
				test.Threshold,
				&buf,
			)

			if test.WantErr && err != nil {
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			output := buf.String()
			if len(output) == 0 {
				t.Fatal("Expected non-empty SARIF output")
			}

			// SARIF output should be valid JSON containing the schema URL
			if !bytes.Contains(buf.Bytes(), []byte("sarif")) {
				t.Error("Output does not appear to be SARIF format")
			}
		})
	}
}

func TestGenerateSARIFRunError(t *testing.T) {
	origScRun := scRun
	t.Cleanup(func() { scRun = origScRun })
	scRun = func(_ context.Context, _ clients.Repo, _ ...sc.Option) (sc.Result, error) {
		return sc.Result{}, errTest
	}

	var buf bytes.Buffer
	err := generateSARIF(
		context.Background(),
		nil,
		nil,
		[]string{"Binary-Artifacts"},
		8,
		&buf,
	)
	if err == nil {
		t.Fatal("Expected error from failed scorecard run")
	}
}

func TestCompressAndEncode(t *testing.T) {
	input := []byte(`{"version":"2.1.0","runs":[]}`)

	encoded, err := compressAndEncode(input)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("Expected non-empty encoded output")
	}

	// Decode base64
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("Invalid base64: %v", err)
	}

	// Decompress gzip
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("Invalid gzip: %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read gzip: %v", err)
	}

	if !bytes.Equal(decompressed, input) {
		t.Errorf("Round-trip failed. want %q, got %q", input, decompressed)
	}
}

func TestUploadToCodeScanning(t *testing.T) {
	origUpload := codeScanningUploadFunc
	origGetRef := getDefaultBranchRefFunc
	t.Cleanup(func() {
		codeScanningUploadFunc = origUpload
		getDefaultBranchRefFunc = origGetRef
	})

	var capturedAnalysis *github.SarifAnalysis
	codeScanningUploadFunc = func(_ context.Context, _ *github.Client,
		_, _ string, analysis *github.SarifAnalysis,
	) (*github.SarifID, *github.Response, error) {
		capturedAnalysis = analysis
		return &github.SarifID{ID: github.Ptr("test-id")}, nil, nil
	}
	getDefaultBranchRefFunc = func(_ context.Context, _ *github.Client,
		_, _ string,
	) (ref, sha string, err error) {
		return "refs/heads/main", "abc123", nil
	}

	sarifContent := []byte(`{"version":"2.1.0","runs":[]}`)
	err := uploadToCodeScanning(
		context.Background(),
		github.NewClient(nil),
		"testorg", "testrepo",
		sarifContent,
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if capturedAnalysis == nil {
		t.Fatal("Expected upload to be called")
	}
	if capturedAnalysis.GetCommitSHA() != "abc123" {
		t.Errorf("Expected commit SHA abc123, got %s", capturedAnalysis.GetCommitSHA())
	}
	if capturedAnalysis.GetRef() != "refs/heads/main" {
		t.Errorf("Expected ref refs/heads/main, got %s", capturedAnalysis.GetRef())
	}
	if capturedAnalysis.GetToolName() != "OpenSSF Scorecard" {
		t.Errorf("Expected tool name OpenSSF Scorecard, got %s", capturedAnalysis.GetToolName())
	}
	if capturedAnalysis.GetSarif() == "" {
		t.Error("Expected non-empty SARIF content")
	}
}

func TestUploadToCodeScanningAPIError(t *testing.T) {
	origUpload := codeScanningUploadFunc
	origGetRef := getDefaultBranchRefFunc
	t.Cleanup(func() {
		codeScanningUploadFunc = origUpload
		getDefaultBranchRefFunc = origGetRef
	})

	codeScanningUploadFunc = func(_ context.Context, _ *github.Client,
		_, _ string, _ *github.SarifAnalysis,
	) (*github.SarifID, *github.Response, error) {
		return nil, nil, errTest
	}
	getDefaultBranchRefFunc = func(_ context.Context, _ *github.Client,
		_, _ string,
	) (ref, sha string, err error) {
		return "refs/heads/main", "abc123", nil
	}

	err := uploadToCodeScanning(
		context.Background(),
		github.NewClient(nil),
		"testorg", "testrepo",
		[]byte(`{}`),
	)
	if err == nil {
		t.Fatal("Expected error from failed API call")
	}
}

func TestUploadSARIF(t *testing.T) {
	origScRun := scRun
	origUpload := codeScanningUploadFunc
	origGetRef := getDefaultBranchRefFunc
	t.Cleanup(func() {
		scRun = origScRun
		codeScanningUploadFunc = origUpload
		getDefaultBranchRefFunc = origGetRef
		clearSARIFHashes()
	})

	scRun = func(_ context.Context, _ clients.Repo, _ ...sc.Option) (sc.Result, error) {
		return sc.Result{
			Repo: sc.RepoInfo{Name: "github.com/test/repo"},
			Checks: []checker.CheckResult{
				{Name: "Binary-Artifacts", Score: 10},
			},
		}, nil
	}
	getDefaultBranchRefFunc = func(_ context.Context, _ *github.Client,
		_, _ string,
	) (ref, sha string, err error) {
		return "refs/heads/main", "abc123", nil
	}

	uploadCount := 0
	codeScanningUploadFunc = func(_ context.Context, _ *github.Client,
		_, _ string, _ *github.SarifAnalysis,
	) (*github.SarifID, *github.Response, error) {
		uploadCount++
		return &github.SarifID{ID: github.Ptr("test-id")}, nil, nil
	}

	ctx := context.Background()
	c := github.NewClient(nil)
	checks := []string{"Binary-Artifacts"}

	// First call should upload.
	err := uploadSARIF(ctx, c, "testorg", "testrepo", checks, 8, nil, nil)
	if err != nil {
		t.Fatalf("First upload failed: %v", err)
	}
	if uploadCount != 1 {
		t.Errorf("Expected 1 upload, got %d", uploadCount)
	}

	// Second call with same result should skip (change detection).
	err = uploadSARIF(ctx, c, "testorg", "testrepo", checks, 8, nil, nil)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	if uploadCount != 1 {
		t.Errorf("Expected still 1 upload (skipped), got %d", uploadCount)
	}
}

func TestUploadSARIFDifferentResult(t *testing.T) {
	origScRun := scRun
	origUpload := codeScanningUploadFunc
	origGetRef := getDefaultBranchRefFunc
	t.Cleanup(func() {
		scRun = origScRun
		codeScanningUploadFunc = origUpload
		getDefaultBranchRefFunc = origGetRef
		clearSARIFHashes()
	})

	callNum := 0
	scRun = func(_ context.Context, _ clients.Repo, _ ...sc.Option) (sc.Result, error) {
		callNum++
		return sc.Result{
			Repo: sc.RepoInfo{Name: "github.com/test/repo"},
			Checks: []checker.CheckResult{
				{Name: "Binary-Artifacts", Score: callNum}, // different score each call
			},
		}, nil
	}
	getDefaultBranchRefFunc = func(_ context.Context, _ *github.Client,
		_, _ string,
	) (ref, sha string, err error) {
		return "refs/heads/main", "abc123", nil
	}

	uploadCount := 0
	codeScanningUploadFunc = func(_ context.Context, _ *github.Client,
		_, _ string, _ *github.SarifAnalysis,
	) (*github.SarifID, *github.Response, error) {
		uploadCount++
		return &github.SarifID{ID: github.Ptr("test-id")}, nil, nil
	}

	ctx := context.Background()
	c := github.NewClient(nil)
	checks := []string{"Binary-Artifacts"}

	// First call uploads.
	if err := uploadSARIF(ctx, c, "testorg", "testrepo", checks, 8, nil, nil); err != nil {
		t.Fatalf("First upload failed: %v", err)
	}
	// Second call should also upload because the result changed.
	if err := uploadSARIF(ctx, c, "testorg", "testrepo", checks, 8, nil, nil); err != nil {
		t.Fatalf("Second upload failed: %v", err)
	}
	if uploadCount != 2 {
		t.Errorf("Expected 2 uploads (result changed), got %d", uploadCount)
	}
}

// errTest is a sentinel error for testing.
var errTest = errors.New("test error")
