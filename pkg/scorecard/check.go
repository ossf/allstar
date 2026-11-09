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
	"fmt"

	"github.com/ossf/scorecard/v5/checker"
	"github.com/ossf/scorecard/v5/clients"
	sc "github.com/ossf/scorecard/v5/pkg/scorecard"
)

var errUnexpectedCheckCount = errors.New("expected exactly one Scorecard check result")

// RunCheck runs one Scorecard check and returns an error if it cannot produce a
// usable result. Scanner failures are distinct from policy compliance results.
func RunCheck(ctx context.Context, client *ScClient, name string) (checker.CheckResult, error) {
	return runCheck(ctx, client, name, sc.Run)
}

func runCheck(ctx context.Context, client *ScClient, name string,
	run func(context.Context, clients.Repo, ...sc.Option) (sc.Result, error),
) (checker.CheckResult, error) {
	result, err := run(ctx, client.ScRepo,
		sc.WithRepoClient(client.ScRepoClient),
		sc.WithChecks([]string{name}),
	)
	if err != nil {
		return checker.CheckResult{}, fmt.Errorf("running Scorecard check %s: %w", name, err)
	}
	if len(result.Checks) != 1 {
		return checker.CheckResult{}, fmt.Errorf("%s: %w (got %d)", name, errUnexpectedCheckCount, len(result.Checks))
	}
	check := result.Checks[0]
	if check.Error != nil {
		return checker.CheckResult{}, fmt.Errorf("evaluating Scorecard check %s: %w", name, check.Error)
	}
	return check, nil
}
