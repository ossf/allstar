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
	"os"
	"path/filepath"
	"testing"

	"github.com/gobwas/glob"
	"github.com/google/go-github/v43/github"
	"github.com/ossf/allstar/pkg/config"
	"github.com/rhysd/actionlint"
)

func TestCheck(t *testing.T) {
	createWorkflowRun := func(sha string, complete bool, passing *bool) *github.WorkflowRun {
		status := "completed"
		if !complete {
			status = "queued"
		}
		conc := "failure"
		if passing == nil {
			conc = ""
		} else if *passing {
			conc = "success"
		}
		return &github.WorkflowRun{
			HeadSHA:    &sha,
			Status:     &status,
			Conclusion: &conc,
		}
	}

	boolptr := func(b bool) *bool {
		return &b
	}

	type testingWorkflowMetadata struct {
		// File is the actual filename of the workflow to load.
		// Will be loaded from test_workflows/ directory.
		File string

		Runs []*github.WorkflowRun
	}

	type testingCommitMetadata struct {
		SHA string

		Tag    []string
		Branch []string

		Parents []string
	}

	denyAll := &Rule{
		Name:   "Deny default",
		Method: "deny",
	}

	requireGoAction := &Rule{
		Name:   "Require mandatory Go Actions",
		Method: "require",
		Actions: []*ActionSelector{
			{
				Name:    "ossf/go-action",
				Version: "commit-ref-1",
			},
		},
		RequireAll: true,
	}

	commitsWrapperValidateWorkflow := map[string][]*testingCommitMetadata{
		"actions/checkout": {
			{
				SHA:     "Commit-A-1",
				Tag:     []string{"v2.0.0"},
				Parents: []string{"Commit-A-1"},
			},
		},
		"gradle/wrapper-validation-action": {
			{
				SHA:     "Commit-B-4",
				Tag:     []string{},
				Parents: []string{"Commit-B-3"},
			},
			{
				SHA:     "Commit-B-3",
				Tag:     []string{"v2.0.0"},
				Parents: []string{"Commit-B-2"},
			},
			{
				SHA:     "Commit-B-2",
				Tag:     []string{"v1.0.4"},
				Parents: []string{"Commit-B-1"},
			},
			{
				SHA:     "Commit-B-1",
				Tag:     []string{"v1.0.0"},
				Parents: []string{"Commit-B-1"},
			},
		},
	}

	tests := []struct {
		Name string

		Org OrgConfig

		// Workflows is a map of filenames to workflowMetadata structs.
		// Filename: just filename eg. "my_workflow.yaml"
		Workflows []testingWorkflowMetadata

		LatestCommitHash string

		Langs map[string]int

		// Commits is a map of "owner/repo" to list of *testingCommitMetadata
		Commits map[string][]*testingCommitMetadata

		ExpectMessage []string
		ExpectPass    bool
	}{
		{
			Name: "Deny all, has Action",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							denyAll,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			ExpectPass:    false,
			ExpectMessage: []string{"denied by deny rule \"Deny default\""},
		},
		{
			Name: "Deny all, no Action",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							denyAll,
						},
					},
				},
			},
			Workflows:  []testingWorkflowMetadata{},
			ExpectPass: true,
		},
		{
			Name: "Deny all, no Action (but Workflow present)",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							denyAll,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "actionless.yaml",
				},
			},
			ExpectPass: true,
		},
		{
			Name: "Deny all, Action present",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							denyAll,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			ExpectPass: false,
		},
		{
			Name: "Deny some, repo match",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Repos: []*RepoSelector{
							{
								Name: "*",
							},
						},
						Rules: []*Rule{
							{
								Name:   "Deny some",
								Method: "deny",
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			ExpectPass:    false,
			ExpectMessage: []string{"denied by deny rule \"Deny some\""},
		},
		{
			Name: "Deny some, repo no match due to exclusion",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Repos: []*RepoSelector{
							{
								Name: "*",
								Exclude: []*RepoSelector{
									{
										Name: "t*srepo",
									},
								},
							},
						},
						Rules: []*Rule{
							{
								Name:   "Deny some",
								Method: "deny",
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			ExpectPass: true,
		},
		{
			Name: "Allowlist new versions, new version",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Allowlist trusted Actions",
								Method: "allow",
								Actions: []*ActionSelector{
									{
										Name: "actions/*",
									},
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v1.0.0",
									},
								},
							},
							denyAll,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
				},
			},
			Commits:    commitsWrapperValidateWorkflow,
			ExpectPass: true,
		},
		{
			Name: "Allowlist new versions, old version",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Allowlist trusted Actions",
								Method: "allow",
								Actions: []*ActionSelector{
									{
										Name: "actions/*",
									},
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v2.0.0",
									},
								},
							},
							denyAll,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
				},
			},
			Commits:    commitsWrapperValidateWorkflow,
			ExpectPass: false,
			ExpectMessage: []string{
				"does not meet version requirement \">= v2.0.0\" for allow rule \"Allowlist",
				"denied by deny rule \"Deny default\"",
			},
		},
		{
			Name: "Allowlist lt newer version",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Allowlist trusted Actions",
								Method: "allow",
								Actions: []*ActionSelector{
									{
										Name: "actions/*",
									},
									{
										Name:    "gradle/wrapper-validation-action",
										Version: "< v2.0.0",
									},
								},
							},
							denyAll,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
				},
			},
			Commits:    commitsWrapperValidateWorkflow,
			ExpectPass: true,
		},
		{
			Name: "Allowlist lt and gt current version",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Allowlist trusted Actions",
								Method: "allow",
								Actions: []*ActionSelector{
									{
										Name: "actions/*",
									},
									{
										Name:    "gradle/wrapper-validation-action",
										Version: "< v1.0.4",
									},
									{
										Name:    "gradle/wrapper-validation-action",
										Version: "> v1.0.4",
									},
								},
							},
							denyAll,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
				},
			},
			Commits:    commitsWrapperValidateWorkflow,
			ExpectPass: false,
		},
		{
			Name: "Allowlist lte current version",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Allowlist trusted Actions",
								Method: "allow",
								Actions: []*ActionSelector{
									{
										Name: "actions/*",
									},
									{
										Name:    "gradle/wrapper-validation-action",
										Version: "<= v1.0.4",
									},
								},
							},
							denyAll,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
				},
			},
			Commits:    commitsWrapperValidateWorkflow,
			ExpectPass: true,
		},
		{
			Name: "Allowlist new versions, new version and old version",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Name: "RG1",
						Rules: []*Rule{
							{
								Name:   "Allowlist trusted Actions",
								Method: "allow",
								Actions: []*ActionSelector{
									{
										Name: "actions/*",
									},
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v1.0.4",
									},
								},
							},
							denyAll,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
				},
				{
					File: "gradle-wrapper-validate-outdated.yaml",
				},
			},
			Commits:    commitsWrapperValidateWorkflow,
			ExpectPass: false,
			ExpectMessage: []string{
				`version v1.0.0 hit deny rule "Deny default"`,
				`does not meet version requirement ">= v1.0.4" * (member of rule group "RG1")`,
				`denied by deny rule "Deny default" (member of rule group "RG1")`,
			},
		},
		{
			Name: "Require new version, new version",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Require Gradle Wrapper validation",
								Method: "require",
								Actions: []*ActionSelector{
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v1.0.4",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
				},
			},
			Commits:    commitsWrapperValidateWorkflow,
			ExpectPass: true,
		},
		{
			Name: "Require new version by tag, use untagged new version",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Require Gradle Wrapper validation",
								Method: "require",
								Actions: []*ActionSelector{
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v1.0.4",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate-version-new-untagged.yaml",
				},
			},
			Commits:    commitsWrapperValidateWorkflow,
			ExpectPass: false,
		},
		{
			Name: "Require new version by commit, use untagged new version",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Require Gradle Wrapper validation",
								Method: "require",
								Actions: []*ActionSelector{
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= Commit-B-3",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate-version-new-untagged.yaml",
				},
			},
			Commits:    commitsWrapperValidateWorkflow,
			ExpectPass: true,
		},
		{
			Name: "Require new version, old version",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Require Gradle Wrapper validation",
								Method: "require",
								Actions: []*ActionSelector{
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v2.0.0",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
				},
			},
			Commits:    commitsWrapperValidateWorkflow,
			ExpectPass: false,
			ExpectMessage: []string{
				"Require rule \"Require Gradle * not satisfied",
				"0 / 1 requisites met",
				"Update *\"gradle/wrapper-val*\" to version satisfying \">= v2.0.0\"",
			},
		},
		{
			Name: "Require multiple, all present",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:       "Required Actions",
								Method:     "require",
								RequireAll: true,
								Actions: []*ActionSelector{
									{
										Name: "gradle/wrapper-validation-action",
									},
									{
										Name: "ossf/go-action",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
				},
				{
					File: "go-workflow.yaml",
				},
			},
			ExpectPass: true,
		},
		{
			Name: "Require multiple, one present",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:       "Required Actions",
								Method:     "require",
								RequireAll: true,
								Actions: []*ActionSelector{
									{
										Name: "gradle/wrapper-validation-action",
									},
									{
										Name: "ossf/go-action",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
				},
			},
			ExpectPass: false,
			ExpectMessage: []string{
				`1 / 2 requisites met`,
				`Add Action "ossf/go-action"`,
			},
		},
		{
			Name: "Require passing, passing on latest",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:     "Require Gradle Wrapper validation",
								Method:   "require",
								MustPass: true,
								Actions: []*ActionSelector{
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v1.0.4",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
					Runs: []*github.WorkflowRun{
						createWorkflowRun("sha-latest", true, boolptr(true)),
					},
				},
			},
			Commits:          commitsWrapperValidateWorkflow,
			LatestCommitHash: "sha-latest",
			ExpectPass:       true,
		},
		{
			Name: "Require passing, pending on latest",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:     "Require Gradle Wrapper validation",
								Method:   "require",
								MustPass: true,
								Actions: []*ActionSelector{
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v1.0.4",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
					Runs: []*github.WorkflowRun{
						createWorkflowRun("sha-latest", false, nil),
					},
				},
			},
			Commits:          commitsWrapperValidateWorkflow,
			LatestCommitHash: "sha-latest",
			ExpectPass:       true,
		},
		{
			Name: "Require passing, passing only on old commit",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:     "Require Gradle Wrapper validation",
								Method:   "require",
								MustPass: true,
								Actions: []*ActionSelector{
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v1.0.4",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
					Runs: []*github.WorkflowRun{
						createWorkflowRun("sha-old", true, boolptr(true)),
					},
				},
			},
			LatestCommitHash: "sha-latest",
			Commits:          commitsWrapperValidateWorkflow,
			ExpectPass:       false,
			ExpectMessage: []string{
				`Require rule "Require * not satisfied`,
				`0 / 1 requisites met`,
				`Fix non-passing Action "gradle/wrapper*" in workflow "GW Validate`,
			},
		},
		{
			Name: "Require passing, failing on current commit",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:     "Require Gradle Wrapper validation",
								Method:   "require",
								MustPass: true,
								Actions: []*ActionSelector{
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v1.0.4",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "gradle-wrapper-validate.yaml",
					Runs: []*github.WorkflowRun{
						createWorkflowRun("sha-latest", true, boolptr(false)),
					},
				},
			},
			LatestCommitHash: "sha-latest",
			Commits:          commitsWrapperValidateWorkflow,
			ExpectPass:       false,
			ExpectMessage: []string{
				"Require rule \"Require * not satisfied",
				"0 / 1 requisites met",
				"Fix non-passing * \"gradle/wrapper*\"",
			},
		},
		{
			Name: "Require, not present",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Require Gradle Wrapper validation",
								Method: "require",
								Actions: []*ActionSelector{
									{
										Name:    "gradle/wrapper-validation-action",
										Version: ">= v1.0.4",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			LatestCommitHash: "sha-latest",
			ExpectPass:       false,
			ExpectMessage: []string{
				"Require rule \"Require * not satisfied",
				"0 / 1 requisites met",
				"Add Action \"gradle/wrapper*\" with version satisfying \">= v1.0.4\"",
			},
		},
		{
			Name: "Require for lang, present",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Name: "Go repos",
						Repos: []*RepoSelector{
							{
								Language: []string{"go"},
							},
						},
						Rules: []*Rule{
							requireGoAction,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "go-workflow.yaml",
				},
			},
			Langs: map[string]int{
				"go": 1000,
			},
			ExpectPass:    true,
			ExpectMessage: []string{},
		},
		{
			Name: "Require for lang, missing",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Name: "Go repos",
						Repos: []*RepoSelector{
							{
								Language: []string{"go"},
							},
						},
						Rules: []*Rule{
							requireGoAction,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			Langs: map[string]int{
				"go": 1000,
			},
			ExpectPass: false,
			ExpectMessage: []string{
				`Require rule "Require mandatory Go Actions* (member of rule group "Go repos* not satisfied`,
				`Add Action "ossf/go-action" with version satisfying "commit-ref-1"`,
			},
		},
		{
			Name: "Require for another lang, missing",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Name: "Go repos",
						Repos: []*RepoSelector{
							{
								Language: []string{"go"},
							},
						},
						Rules: []*Rule{
							requireGoAction,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			Langs: map[string]int{
				"ts": 1000,
			},
			ExpectPass: true,
		},
		{
			Name: "Require for non-top but significant lang, missing",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Name: "Go repos",
						Repos: []*RepoSelector{
							{
								Language: []string{"go"},
							},
						},
						Rules: []*Rule{
							requireGoAction,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			Langs: map[string]int{
				"c":  7000,
				"go": 5000,
			},
			ExpectPass: false,
		},
		{
			Name: "Require for insignificant lang, missing",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Name: "Go repos",
						Repos: []*RepoSelector{
							{
								Language: []string{"go"},
							},
						},
						Rules: []*Rule{
							requireGoAction,
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			Langs: map[string]int{
				"c":  7000,
				"go": 60,
			},
			ExpectPass: true,
		},
		{
			Name: "Require Action with semver constraint, but it is commit-pinned in repo",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Name: "All repos",
						Rules: []*Rule{
							{
								Name:   "Require some action",
								Method: "require",
								Actions: []*ActionSelector{
									{
										Name:    "ossf/test-action",
										Version: ">= v1.0.0",
									},
								},
							},
						},
					},
				},
			},
			Commits: map[string][]*testingCommitMetadata{
				"ossf/test-action": {
					{
						SHA:     "696c241da8ea301b3f1d2343c45c1e4aa38f90c7",
						Tag:     []string{"v1.2.0"},
						Parents: []string{"another sha"},
					},
					{
						SHA: "another sha",
						Tag: []string{"v1.0.0"},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "version-pinned.yaml",
				},
			},
			ExpectPass: true,
		},
		{
			Name: "Require Action with semver constraint, and old version is commit-pinned in repo",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Name: "All repos",
						Rules: []*Rule{
							{
								Name:   "Require some action",
								Method: "require",
								Actions: []*ActionSelector{
									{
										Name:    "ossf/test-action",
										Version: ">= v1.2.0",
									},
								},
							},
						},
					},
				},
			},
			Commits: map[string][]*testingCommitMetadata{
				"ossf/test-action": {
					{
						SHA:     "another sha",
						Tag:     []string{"v1.2.0"},
						Parents: []string{"696c241da8ea301b3f1d2343c45c1e4aa38f90c7"},
					},
					{
						SHA:     "696c241da8ea301b3f1d2343c45c1e4aa38f90c7",
						Tag:     []string{"v0.9.0"},
						Parents: []string{"696c241da8ea301b3f1d2343c45c1e4aa38f90c7"},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "version-pinned.yaml",
				},
			},
			ExpectPass: false,
			ExpectMessage: []string{
				`Update Action "ossf/test-action" to * ">= v1.2.0"`,
			},
		},
		{
			Name: "Deny higher priority than allow",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:     "Allow all",
								Method:   "allow",
								Priority: "medium",
							},
							{
								Name:     "Deny all",
								Method:   "deny",
								Priority: "high",
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			ExpectPass:    false,
			ExpectMessage: []string{"denied by deny rule \"Deny all\""},
		},
		{
			Name: "Deny same priority as allow",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:     "Allow all",
								Method:   "allow",
								Priority: "high",
							},
							{
								Name:     "Deny all",
								Method:   "deny",
								Priority: "high",
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			ExpectPass: true,
		},
		{
			Name: "Deny lower priority than allow",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:     "Allow all",
								Method:   "allow",
								Priority: "high",
							},
							{
								Name:     "Deny all",
								Method:   "deny",
								Priority: "low",
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "basic.yaml",
				},
			},
			ExpectPass: true,
		},
		{
			Name: "Invalid workflow present",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:     "Allow all",
								Method:   "allow",
								Priority: "high",
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "invalid.yaml",
				},
			},
			ExpectPass: true,
		},
		{
			Name: "Invalid workflow present along with workflow with denied Action",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Deny ossf/test-action",
								Method: "deny",
								Actions: []*ActionSelector{
									{
										Name: "ossf/test-action",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "invalid.yaml",
				},
				{
					File: "version-pinned.yaml",
				},
			},
			ExpectPass: false,
		},
		{
			Name: "Required Action present in workflow without correct \"on\" values",
			Org: OrgConfig{
				Action: "issue",
				Groups: []*RuleGroup{
					{
						Rules: []*Rule{
							{
								Name:   "Require ossf/required-action",
								Method: "require",
								Actions: []*ActionSelector{
									{
										Name: "ossf/required-action",
									},
								},
							},
						},
					},
				},
			},
			Workflows: []testingWorkflowMetadata{
				{
					File: "no-on.yaml",
				},
			},
			ExpectPass: false,
			ExpectMessage: []string{
				`Enable workflow "Test Workflow 2" containing Action "oss* to run on pull_request and push.`,
			},
		},
	}

	a := NewAction()

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Override external functions

			configFetchConfig = func(ctx context.Context, c *github.Client, owner, repo, path string,
				ol config.ConfigLevel, out interface{}) error {
				if ol == config.OrgLevel {
					oc := out.(*OrgConfig)
					*oc = test.Org
				}
				return nil
			}

			listWorkflows = func(ctx context.Context, c *github.Client, owner, repo string) (
				[]*workflowMetadata, error) {
				var wfs []*workflowMetadata
				for _, w := range test.Workflows {
					d, err := os.ReadFile(filepath.Join("test_workflows", w.File))
					if err != nil {
						return nil, fmt.Errorf("failed to open test workflow file: %w", err)
					}
					workflow, errs := actionlint.Parse(d)
					if len(errs) > 0 {
						for _, er := range errs {
							t.Logf("parse err on %s: %s", w.File, er.Error())
						}
					}
					if workflow == nil {
						t.Logf("nil workflow")
						continue
					}
					wfs = append(wfs, &workflowMetadata{
						filename: w.File,
						workflow: workflow,
					})
				}
				return wfs, nil
			}

			listLanguages = func(ctx context.Context, c *github.Client, owner, repo string) (map[string]int, error) {
				return test.Langs, nil
			}

			listWorkflowRunsByFilename = func(ctx context.Context, c *github.Client, owner, repo,
				workflowFilename string) ([]*github.WorkflowRun, error) {
				for _, wf := range test.Workflows {
					if wf.File == workflowFilename {
						return wf.Runs, nil
					}
				}
				return nil, fmt.Errorf("could not find testWorkflowMetadata for filename %s", workflowFilename)
			}

			getLatestCommitHash = func(ctx context.Context, c *github.Client, owner, repo string) (string, error) {
				return test.LatestCommitHash, nil
			}

			listTags = func(ctx context.Context, c *github.Client, owner, repo string) ([]*github.RepositoryTag, error) {
				ownerRepo := fmt.Sprintf("%s/%s", owner, repo)
				var tags []*github.RepositoryTag
				commits, ok := test.Commits[ownerRepo]
				if !ok {
					t.Logf("tried to find tags for \"%s\", but commits were not specified in test", ownerRepo)
					return nil, nil
				}
				for _, c := range commits {
					for _, t := range c.Tag {
						tags = append(tags, &github.RepositoryTag{
							Name: &t,
							Commit: &github.Commit{
								SHA: &c.SHA,
							},
						})
					}
				}
				return tags, nil
			}

			listBranches = func(ctx context.Context, c *github.Client, owner, repo string) ([]*github.Branch, error) {
				ownerRepo := fmt.Sprintf("%s/%s", owner, repo)
				var branches []*github.Branch
				commits, ok := test.Commits[ownerRepo]
				if !ok {
					t.Logf("tried to find branches for \"%s\", but commits were not specified in test", ownerRepo)
					return nil, nil
				}
				for _, c := range commits {
					for _, t := range c.Branch {
						branches = append(branches, &github.Branch{
							Name: &t,
							Commit: &github.RepositoryCommit{
								SHA: &c.SHA,
							},
						})
					}
				}
				return branches, nil
			}

			listCommits = func(ctx context.Context, c *github.Client, owner, repo string) ([]*github.RepositoryCommit, error) {
				ownerRepo := fmt.Sprintf("%s/%s", owner, repo)
				var commits []*github.RepositoryCommit
				cms, ok := test.Commits[ownerRepo]
				if !ok {
					t.Logf("tried to find commits for \"%s\", but commits were not specified in test", ownerRepo)
					return nil, nil
				}
				for _, c := range cms {
					var parents []*github.Commit
					for _, p := range c.Parents {
						parents = append(parents, &github.Commit{
							SHA: &p,
						})
					}
					commits = append(commits, &github.RepositoryCommit{
						SHA:     &c.SHA,
						Parents: parents,
					})
				}
				return commits, nil
			}

			res, err := a.Check(context.Background(), nil, "thisorg", "thisrepo")

			// Check result

			if err != nil {
				t.Errorf("Error: %e", err)
			}

			if res.Pass != test.ExpectPass {
				t.Errorf("Expect pass = %t, got pass = %t", test.ExpectPass, res.Pass)
				t.Logf("NotifyText:\n%s", res.NotifyText)
			}

			if !res.Pass {
				d := res.Details.(details)
				if d.FailedRules == nil {
					t.Errorf("FailedRules nil")
				}
				for i, r := range d.FailedRules {
					if r == nil {
						t.Errorf("nil Rule in FailedRules")
					}
					for i2, r2 := range d.FailedRules {
						if i != i2 && r == r2 {
							t.Errorf("duplicate Rule in FailedRules")
						}
					}
				}
			}

			for _, message := range test.ExpectMessage {
				comp, err := glob.Compile("*" + message + "*")
				if err != nil {
					t.Errorf("failed to parse ExpectMessage glob: %s", err.Error())
				}
				if !comp.Match(res.NotifyText) {
					t.Errorf("\"%s\" does not contain \"%s\"", res.NotifyText, message)
				}
			}
		})
	}
}
