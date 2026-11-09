// Copyright 2026 Allstar Authors

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

	"github.com/google/go-cmp/cmp"

	"github.com/ossf/scorecard/v5/checker"
	"github.com/ossf/scorecard/v5/clients"
	sc "github.com/ossf/scorecard/v5/pkg/scorecard"
)

func TestRunCheckResults(t *testing.T) {
	t.Parallel()
	scanErr := errors.New("scanner unavailable")
	passing := checker.CheckResult{Name: "test-check", Score: 10}
	failing := checker.CheckResult{Name: "test-check", Score: 0}
	tests := []struct {
		name    string
		result  sc.Result
		runErr  error
		want    checker.CheckResult
		wantErr error
	}{
		{name: "passing", result: sc.Result{Checks: []checker.CheckResult{passing}}, want: passing},
		{name: "noncompliant", result: sc.Result{Checks: []checker.CheckResult{failing}}, want: failing},
		{name: "run failure", runErr: scanErr, wantErr: scanErr},
		{
			name:    "per-check failure",
			result:  sc.Result{Checks: []checker.CheckResult{{Name: "test-check", Error: scanErr}}},
			wantErr: scanErr,
		},
		{name: "no checks", wantErr: errUnexpectedCheckCount},
		{
			name:    "multiple checks",
			result:  sc.Result{Checks: []checker.CheckResult{passing, failing}},
			wantErr: errUnexpectedCheckCount,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			run := func(context.Context, clients.Repo, ...sc.Option) (sc.Result, error) {
				return tt.result, tt.runErr
			}
			got, err := runCheck(t.Context(), &ScClient{}, "test-check", run)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Error = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Unexpected check result. (-want +got):\n%s", diff)
			}
		})
	}
}
