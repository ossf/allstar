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
	"context"
	"errors"
	"testing"

	"github.com/google/go-github/v84/github"

	"github.com/ossf/scorecard/v5/checker"
	sc "github.com/ossf/scorecard/v5/pkg/scorecard"
)

// A daemon never writes the results file, so uploadSARIF must not keep
// results unless collection was turned on. Uploads that keep failing (for
// example a 403 while the App lacks security_events: write) retry the same
// commit every cycle, which used to add an entry each time.
func TestResultsCollection(t *testing.T) {
	origUpload := codeScanningUploadFunc
	origGetRef := getDefaultBranchRefFunc
	resetState := func(collect bool) {
		clearSARIFHashes()
		resultsMu.Lock()
		resultsEntries = nil
		collectResults = collect
		resultsMu.Unlock()
	}
	t.Cleanup(func() {
		codeScanningUploadFunc = origUpload
		getDefaultBranchRefFunc = origGetRef
		resetState(false)
	})

	getDefaultBranchRefFunc = func(_ context.Context, _ *github.Client,
		_, _ string,
	) (string, string, error) {
		return "refs/heads/main", "samecommit", nil
	}
	codeScanningUploadFunc = func(_ context.Context, _ *github.Client,
		_, _ string, _ *github.SarifAnalysis,
	) (*github.SarifID, *github.Response, error) {
		return nil, nil, errors.New("403 Resource not accessible by integration")
	}

	result := &sc.Result{
		Repo:   sc.RepoInfo{Name: "github.com/testorg/testrepo"},
		Checks: []checker.CheckResult{{Name: "Binary-Artifacts", Score: 10}},
	}

	tests := []struct {
		Name    string
		Collect bool
		Cycles  int
		Want    int
	}{
		{Name: "DaemonKeepsNothing", Collect: false, Cycles: 50, Want: 0},
		{Name: "OnceKeepsResult", Collect: true, Cycles: 1, Want: 1},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			resetState(test.Collect)
			for range test.Cycles {
				if err := uploadSARIF(context.Background(), github.NewClient(nil),
					"testorg", "testrepo", result, []string{"Binary-Artifacts"}, 8); err == nil {
					t.Fatal("Expected upload error")
				}
			}
			resultsMu.Lock()
			got := len(resultsEntries)
			resultsMu.Unlock()
			if got != test.Want {
				t.Errorf("Expected %d collected results after %d cycles, got %d",
					test.Want, test.Cycles, got)
			}
		})
	}
}
