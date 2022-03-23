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

// Package policydef defines the interface that policies must implement to be
// included in Allstar.
//
// Policies should define and retrieve their own config in the same way that
// Allstar does. There should be an org-level config and repo-level
// config. Each config should include the OptConfig defined in
// github.com/ossf/allstar/pkg/config to determine if the policy is enabled or
// disabled. The config package also provided helper functions to retreive
// config from the repo.
package policydef

import (
	"context"

	"github.com/google/go-github/v43/github"
)

// Result is returned from a policy check.
type Result struct {
	// Enabled is whether the policy is enabled or not.
	Enabled bool

	// Pass is whether the policy passes or not.
	Pass bool

	// NotifyText is the human readable message to provide to the user if the
	// configured action is a notify action (issue, email, rpc). It should inform
	// the user of the problem and how to fix it.
	NotifyText string

	// Details are logged on failure. it should be serailizable to json and allow
	// useful log querying.
	Details interface{}
}

// Policy is the interface that policies must implement to be included in
// Allstar.
type Policy interface {

	// Name must return the human readable name of the policy.
	Name() string

	// Check checks whether the provided repo is in compliance with the policy or
	// not. It must use the provided context and github client. See Result for
	// more details on the return value.
	Check(ctx context.Context, c *github.Client, owner, repo string) (*Result, error)

	// Fix should modify the provided repo to be in compliance with the
	// policy. The provided github client must be used to either edit repo
	// settings or modify files. Fix is optional and the policy may simply
	// return.
	Fix(ctx context.Context, c *github.Client, owner, repo string) error

	// GetAction must return the configured action from the policy's config. No
	// validation is needed by the policy, it will be done centrally.
	GetAction(ctx context.Context, c *github.Client, owner, repo string) string
}
