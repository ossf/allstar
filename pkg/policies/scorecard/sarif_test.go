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
	"context"
	"errors"
	"testing"

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

// errTest is a sentinel error for testing.
var errTest = errors.New("test error")
