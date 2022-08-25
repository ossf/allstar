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

// Package action implements the GitHub Actions security policy.
package action

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/ossf/allstar/pkg/config"
	"github.com/ossf/allstar/pkg/policydef"
	"github.com/rhysd/actionlint"

	"github.com/google/go-github/v43/github"
	"github.com/rs/zerolog/log"
)

const configFile = "actions.yaml"
const polName = "GitHub Actions"

var actionNameVersionRegex = regexp.MustCompile(`^([a-zA-Z0-9_\-.]+\/[a-zA-Z0-9_\-.]+)@([a-zA-Z0-9\-.]+)$`)

const failText = "This policy, specified at the organization level, sets requirements for Action use by repos within the organization. This repo is failing to fully comply with organization policies, as explained below.\n\n```\n%s```\n\nSee the org-level %s policy configuration for details."

const maxWorkflows = 50
const repoSelectorExcludeDepthLimit = 3

var priorities = map[string]int{
	"critical": 0,
	"high":     1,
	"medium":   2,
	"low":      3,
}

// OrgConfig is the org-level config definition for Action Use
type OrgConfig struct {
	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`

	// Groups is the set of RuleGroups to employ during Check.
	// They are evaluated in order.
	Groups []*RuleGroup `json:"groups"`
}

// RuleGroup is used to apply rules to repos matched by RepoSelectors.
type RuleGroup struct {
	// Name is the name used to identify the RuleGroup.
	Name string `json:"name"`

	// Repos is the set of RepoSelectors to use when deciding whether a repo
	// qualifies for this RuleGroup.
	// if nil, select all repos.
	Repos []*RepoSelector `json:"repos"`

	// Rules is the set of rules to apply for this RuleGroup.
	// Rules are applied in order of priority, with allow/require rules
	// evaluated before deny rules at each priority tier.
	Rules []*Rule `json:"rules"`
}

// Rule is an Action Use rule
type Rule struct {
	// Name is the name used to identify the rule
	Name string `json:"name"`

	// Method is the type of rule. One of "require", "allow", and "deny".
	Method string `json:"method"`

	// Priority is the priority tier identifier applied to the rule.
	// Options are "urgent", "high", "medium", and "low"
	Priority string `json:"priority"`

	// Actions is a set of ActionSelectors.
	// If nil, all Actions will be selected
	Actions []*ActionSelector `json:"actions"`

	// MustPass specifies whether the rule's Action(s) are required to
	// be part of a passing workflow on latest commit.
	// [For use with "require" method]
	MustPass bool `json:"mustPass"`

	// RequireAll specifies that all Actions listed should be required,
	// rather than just one.
	// [For use with "require" method]
	RequireAll bool `json:"requireAll"`
}

// RepoSelector specifies a selection of repos
type RepoSelector struct {
	// Name is the repo name in glob format
	Name string `json:"name"`

	// Language is a set of programming languages.
	// See the section about language detection below
	Language []string `json:"language"`

	// Exclude is a set of RepoSelectors targeting repos that should
	// not be matched by this selector.
	Exclude []*RepoSelector `json:"exclude"`
}

// ActionSelector specifies a selection of Actions
type ActionSelector struct {
	// Name is the Action name in glob format
	Name string `json:"name"`

	// Version is a semver condition or commit ref
	// Default "" targets any version
	Version string `json:"version"`
}

type details struct {
	FailedRules []*Rule
}

type workflowMetadata struct {
	filename string
	workflow *actionlint.Workflow
}

type actionMetadata struct {
	name             string
	version          string
	workflowFilename string
	workflowName     string
	workflowOn       []actionlint.Event
}

// internalRuleGroup is a RuleGroup using internalRule
type internalRuleGroup struct {
	*RuleGroup

	// Rules is the set of rules to apply for this RuleGroup.
	// Rules are applied in order of priority, with allow/require rules
	// evaluated before deny rules at each priority tier.
	Rules []*internalRule `json:"rules"`
}

// internalRule is an Action Use rule with added internal fields
type internalRule struct {
	*Rule

	// group references the RuleGroup to which this rule belongs
	group *RuleGroup

	// priorityInt is an int corresponding to Priority.
	// Lower value = higher priority
	priorityInt int
}

// internalOrgConfig is the org-level Actions policy config with internalGroup
type internalOrgConfig struct {
	// Action defines which action to take, default log, other: issue...
	Action string `json:"action"`

	// Groups is the set of RuleGroups to employ during Check.
	// They are evaluated in order.
	Groups []*internalRuleGroup `json:"groups"`
}

var configFetchConfig func(context.Context, *github.Client, string, string, string, config.ConfigLevel, interface{}) error

var listWorkflows func(ctx context.Context, c *github.Client, owner, repo string) ([]*workflowMetadata, error)
var listLanguages func(ctx context.Context, c *github.Client, owner, repo string) (map[string]int, error)
var listWorkflowRunsByFilename func(ctx context.Context, c *github.Client, owner, repo string, workflowFilename string) ([]*github.WorkflowRun, error)
var getLatestCommitHash func(ctx context.Context, c *github.Client, owner, repo string) (string, error)
var listReleases func(ctx context.Context, c *github.Client, owner, repo string) ([]*github.RepositoryRelease, error)

func init() {
	configFetchConfig = config.FetchConfig
	listWorkflows = listWorkflowsReal
	listLanguages = listLanguagesReal
	listWorkflowRunsByFilename = listWorkflowRunsByFilenameReal
	getLatestCommitHash = getLatestCommitHashReal
	listReleases = listReleasesReal
}

// sortableRules is a sortable list of *Rule
type sortableRules []*internalRule

// Action is the Action Use policy object, implements policydef.Policy.
type Action bool

// NewAction returns a new Action Use policy.
func NewAction() policydef.Policy {
	var a Action
	return a
}

// Name returns the name of this policy, implementing policydef.Policy.Name()
func (a Action) Name() string {
	return polName
}

// Check performs the policy check for Action Use policy based on the
// configuration stored in the org, implementing policydef.Policy.Check()
func (a Action) Check(ctx context.Context, c *github.Client, owner,
	repo string) (*policydef.Result, error) {
	oc := getConfig(ctx, c, owner, repo)
	enabled := oc.Groups != nil
	log.Info().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Bool("enabled", enabled).
		Msg("Check repo enabled")
	if !enabled {
		// Don't run this policy if no rules exist.
		return &policydef.Result{
			Enabled:    enabled,
			Pass:       true,
			NotifyText: "Disabled",
			Details:    details{},
		}, nil
	}
	// Get workflows.
	// Workflows should have push and pull_request listed as trigger events
	// in order to qualify.
	wfs, err := listWorkflows(ctx, c, owner, repo)
	if err != nil {
		return nil, err
	}

	// Create index of which workflows run which Actions
	var actions []*actionMetadata

	for _, wf := range wfs {
		if wf.workflow.Name == nil {
			wf.workflow.Name = &actionlint.String{Value: wf.filename}
		}
		if wf.workflow.Jobs == nil {
			continue
		}
		for _, j := range wf.workflow.Jobs {
			if j == nil {
				continue
			}
			for _, s := range j.Steps {
				if s == nil || s.Exec == nil {
					continue
				}
				actionStep, ok := s.Exec.(*actionlint.ExecAction)
				if !ok || actionStep == nil {
					continue
				}
				if actionStep.Uses == nil {
					// Missing uses in step
					continue
				}
				sm := actionNameVersionRegex.FindStringSubmatch(actionStep.Uses.Value)
				if sm == nil {
					// Ignore invalid Action
					log.Warn().
						Str("org", owner).
						Str("repo", repo).
						Str("area", polName).
						Str("action", actionStep.Uses.Value).
						Msg("Ignoring invalid action")
					continue
				}
				name := sm[1]
				version := sm[2]
				actions = append(actions, &actionMetadata{
					name:             name,
					version:          version,
					workflowFilename: wf.filename,
					workflowName:     wf.workflow.Name.Value,
					workflowOn:       wf.workflow.On,
				})
			}
		}
	}

	// Init caches

	gc := newGlobCache()
	sc := newSemverCache()

	// Determine applicable rules

	var applicableRules sortableRules

	for _, g := range oc.Groups {
		// Check if group match
		groupMatch := false
		for _, rs := range g.Repos {
			// Ignore error while checking match. Match will be false on error.
			match, err := rs.match(ctx, c, owner, repo, repoSelectorExcludeDepthLimit, gc, sc)

			if err != nil {
				log.Warn().
					Str("org", owner).
					Str("repo", repo).
					Str("area", polName).
					Err(err).
					Msg("Invalid RepoSelector, will skip.")
				continue
			}

			if match {
				groupMatch = true
				break
			}
		}
		if g.Repos == nil {
			groupMatch = true
		}
		if groupMatch {
			applicableRules = append(applicableRules, g.Rules...)
		}
	}

	// Sort rules into priority order
	// First by priority, second by method with require/allow before deny

	sort.Sort(applicableRules)

	// Evaluate rules using index

	var results []ruleEvaluationResult

	// => First, evaluate deny rules
	// Note: deny rules are evaluated Action-wise

	for _, a := range actions {
		denyResult, errors := evaluateActionDenied(ctx, c, applicableRules, a, gc, sc)
		// errors are often parse errors (user-created) and are reflected in
		// denyResult steps

		if errors != nil {
			log.Warn().
				Str("org", owner).
				Str("repo", repo).
				Str("area", polName).
				Str("action", a.name).
				Errs("errors", errors).
				Msg("Errors while evaluating deny rule.")
		}

		results = append(results, denyResult)
	}

	// => Next, evaluate require rules

	var wfr *github.WorkflowRuns
	var headSHA string

	for _, r := range applicableRules {
		if r.Method == "require" {
			if r.MustPass && wfr == nil {
				var err error
				hash, err := getLatestCommitHash(ctx, c, owner, repo)
				if err != nil {
					log.Error().
						Str("org", owner).
						Str("repo", repo).
						Str("area", polName).
						Err(err).
						Msg("Error getting latest commit hash. Skipping rule evaluation.")
					break
				}
				headSHA = hash
			}

			result, err := evaluateRequireRule(ctx, c, owner, repo, r, actions, headSHA, gc, sc)
			if err != nil {
				log.Warn().
					Str("org", owner).
					Str("repo", repo).
					Str("area", polName).
					Err(err).
					Msg("Error evaluating require rule")
				continue
			}
			results = append(results, result)
		}
	}

	d := details{}

	passing := true
	combinedExplain := ""

	// Use this map to dedupe Rules
	failedRules := map[*internalRule]struct{}{}

	for _, result := range results {
		if !result.passed() {
			passing = false
			if combinedExplain != "" {
				combinedExplain += "\n"
			}
			combinedExplain += result.explain()
			failedRules[result.relevantRule()] = struct{}{}
		}
	}

	for r, _ := range failedRules {
		d.FailedRules = append(d.FailedRules, r.Rule)
	}

	notifyText := fmt.Sprintf(failText, combinedExplain, polName)

	if passing {
		notifyText = "OK"
	}

	return &policydef.Result{
		Enabled:    enabled,
		Pass:       passing,
		NotifyText: notifyText,
		Details:    d,
	}, nil
}

// Fix implementing policydef.Policy.Fix(). Not supported.
func (a Action) Fix(ctx context.Context, c *github.Client, owner, repo string) error {
	log.Warn().
		Str("org", owner).
		Str("repo", repo).
		Str("area", polName).
		Msg("Action fix is configured, but not implemented.")
	return nil
}

// GetAction returns the configured action from Action Use policy's
// configuration stored in the org repo, default log. Implementing
// policydef.Policy.GetAction()
func (a Action) GetAction(ctx context.Context, c *github.Client, owner, repo string) string {
	oc := getConfig(ctx, c, owner, repo)
	return oc.Action
}

func getConfig(ctx context.Context, c *github.Client, owner, repo string) *internalOrgConfig {
	oc := &OrgConfig{ // Fill out non-zero defaults
		Action: "log",
	}
	if err := configFetchConfig(ctx, c, owner, "", configFile, config.OrgLevel, oc); err != nil {
		log.Error().
			Str("org", owner).
			Str("repo", repo).
			Str("configLevel", "orgLevel").
			Str("area", polName).
			Str("file", configFile).
			Err(err).
			Msg("Unexpected config error, using defaults.")
	}
	var gs []*internalRuleGroup
	// Initialize values in each rule
	for _, g := range oc.Groups {
		ig := &internalRuleGroup{
			RuleGroup: g,
			Rules:     nil,
		}
		for _, r := range g.Rules {
			ir := &internalRule{Rule: r}
			// Set each rule's group to its *RuleGroup
			ir.group = g
			// Set each rule's priorityInt to an int corresponding to Priority
			if p, ok := priorities[r.Priority]; ok {
				ir.priorityInt = p
			} else {
				ir.priorityInt, ok = priorities["medium"]
				if !ok {
					ir.priorityInt = 2
				}
			}
			ig.Rules = append(ig.Rules, ir)
		}
		gs = append(gs, ig)
	}
	return &internalOrgConfig{
		Action: oc.Action,
		Groups: gs,
	}
}

// resolveVersion gets a *semver.Version given an actionMetadata.
// It will use release tags of the Action repo if necessary.
func resolveVersion(ctx context.Context, c *github.Client, m *actionMetadata, gc globCache, sc semverCache) (*semver.Version, error) {
	version, err := sc.compileVersion(m.version)
	if err == nil {
		return version, nil
	}
	errPrefix := fmt.Sprintf("while resolving version for commit ref \"%s\" in repo \"%s\"", m.version, m.name)
	// On error, attempt locate release tag (assume m.version is a commit ref)
	ownerRepo := strings.Split(m.name, "/")
	if len(ownerRepo) != 2 {
		return nil, fmt.Errorf("%s: invalid name \"%s\"", errPrefix, m.name)
	}
	rels, err := listReleases(ctx, c, ownerRepo[0], ownerRepo[1])
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}
	for _, rel := range rels {
		if rel.GetTargetCommitish() == m.version {
			version, err := sc.compileVersion(rel.GetTagName())
			return version, err
		}
	}
	return nil, fmt.Errorf("%s: no corresponding release found", errPrefix)
}

// match checks if an ActionSelector matches an actionMetadata.
func (as *ActionSelector) match(ctx context.Context, c *github.Client, m *actionMetadata, gc globCache, sc semverCache) (match, matchName, matchVersion bool, err error) {
	if as.Name != "" {
		nameGlob, err := gc.compileGlob(as.Name)
		if err != nil {
			return false, false, false, err
		}
		if !nameGlob.Match(m.name) {
			return false, false, false, nil
		}
	}
	if as.Version == "" {
		return true, true, true, nil
	}
	if as.Version == m.version {
		return true, true, true, nil
	}
	if as.Version != "" {
		constraint, err := sc.compileConstraints(as.Version)
		if err != nil {
			// on error, assume this is a ref
			// (we know it doesn't match because not equal above)
			return false, true, false, nil
		}
		version, err := resolveVersion(ctx, c, m, gc, sc)
		if err != nil {
			return false, true, false, err
		}
		if !constraint.Check(version) {
			return false, true, false, nil
		}
	}
	return true, true, true, nil
}

// match checks if a repo matches a RepoSelector.
// Set excludeDepth to > 0 for exclusion depth limit, or < 0 for no depth limit.
func (rs *RepoSelector) match(ctx context.Context, c *github.Client, owner, repo string, excludeDepth int, gc globCache, sc semverCache) (bool, error) {
	if rs == nil {
		return true, nil
	}
	if rs.Name != "" {
		ng, err := gc.compileGlob(rs.Name)
		if err != nil {
			return false, err
		}
		if !ng.Match(repo) {
			return false, nil
		}
	}
	if rs.Language != nil {
		langs, err := listLanguages(ctx, c, owner, repo)
		if err != nil {
			return false, err
		}
		if !languageSatisfied(langs, rs.Language) {
			return false, nil
		}
	}
	// Check if covered by exclusion case
	if excludeDepth != 0 {
		for _, exc := range rs.Exclude {
			match, err := exc.match(ctx, c, owner, repo, excludeDepth-1, gc, sc)
			if err != nil {
				// API error? Ignore exclusion
				continue
			}
			if match {
				return false, nil
			}
		}
	}
	return true, nil
}

// languageSatisfied determines from a map of languages to bytes whether the
// queried languages are significantly present.
func languageSatisfied(langs map[string]int, want []string) bool {
	totalBytes := 0

	topLangBytes := 0
	topLang := ""

	var significantLanguages []string

	for l, b := range langs {
		totalBytes += b
		if topLang == "" || topLangBytes < b {
			topLang = l
			topLangBytes = b
		}
		if b > 3000 {
			significantLanguages = append(significantLanguages, l)
		}
	}

	significantLanguages = append(significantLanguages, topLang)

	for _, w := range want {
		for _, s := range significantLanguages {
			if strings.EqualFold(s, w) {
				return true
			}
		}
	}

	return false
}

// Len returns number of rules in s
func (s sortableRules) Len() int {
	return len(s)
}

// Less returns i < j, determined by Priority and Method so that priority tiers
// are honored and allow/require is evaluated before deny.
func (s sortableRules) Less(i, j int) bool {
	if s[i].priorityInt < s[j].priorityInt {
		return true
	}
	if s[i].Method != "deny" {
		return true
	}
	return false
}

// Swap swaps the Rules at indices i and j
func (s sortableRules) Swap(i, j int) {
	hold := s[i]
	s[i] = s[j]
	s[j] = hold
}

// getLatestCommitHashReal gets the latest commit hash for a repo's default
// branch using the GitHub API.
// Docs: https://docs.github.com/en/rest/commits/commits#list-commits
func getLatestCommitHashReal(ctx context.Context, c *github.Client, owner, repo string) (string, error) {
	commits, _, err := c.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{})
	if err != nil {
		return "", err
	}
	if len(commits) > 0 && commits[0].SHA != nil {
		return *commits[0].SHA, nil
	}
	return "", fmt.Errorf("repo has no commits: %w", err)
}

// listWorkflowRunsByFilenameReal returns workflow runs for a repo by
// workflow filename.
// Docs:
// https://docs.github.com/en/rest/actions/workflow-runs#list-workflow-runs
func listWorkflowRunsByFilenameReal(ctx context.Context, c *github.Client, owner, repo string, workflowFilename string) ([]*github.WorkflowRun, error) {
	runs, _, err := c.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, workflowFilename, &github.ListWorkflowRunsOptions{
		Event: "push",
	})
	return runs.WorkflowRuns, err
}

// listLanguagesReal uses the GitHub API to list languages.
// Docs: https://docs.github.com/en/rest/repos/repos#list-repository-languages
func listLanguagesReal(ctx context.Context, c *github.Client, owner, repo string) (map[string]int, error) {
	l, _, err := c.Repositories.ListLanguages(ctx, owner, repo)
	return l, err
}

// listWorkflowsReal returns workflows for a repo. If on is specified, will
// filter to workflows with all trigger events listed in on.
// Docs: https://docs.github.com/en/rest/repos/contents#get-repository-content
func listWorkflowsReal(ctx context.Context, c *github.Client, owner, repo string) ([]*workflowMetadata, error) {
	// TODO add cacheable walk to workflows dir here.
	// See pkg/config/contents.go for similar. The difference here is getting
	// dir rather than file contents. Could be nice to modify config's
	// implementation and make it public for use here.
	_, workflowDirContents, resp, err := c.Repositories.GetContents(ctx, owner, repo, ".github/workflows", &github.RepositoryContentGetOptions{})
	if err != nil {
		if resp.StatusCode == 404 {
			// No workflows dir should yield no workflows
			return []*workflowMetadata{}, nil
		}
		return nil, err
	}
	// Limit number of considered workflows to maxWorkflows
	if len(workflowDirContents) > maxWorkflows {
		workflowDirContents = workflowDirContents[:maxWorkflows]
	}
	// Get content for workflows
	for _, wff := range workflowDirContents {
		fc, _, _, err := c.Repositories.GetContents(ctx, owner, repo, wff.GetPath(), &github.RepositoryContentGetOptions{})
		if err != nil {
			return nil, err
		}
		content, err := fc.GetContent()
		if err != nil {
			return nil, err
		}
		wff.Content = &content
	}
	var workflows []*workflowMetadata
	for _, wfc := range workflowDirContents {
		if wfc.Name == nil {
			// missing name?
			log.Error().
				Str("org", owner).
				Str("repo", repo).
				Str("area", polName).
				Str("path", wfc.GetPath()).
				Msg("Workflow file missing name field unexpectedly.")
			continue
		}
		sc, err := wfc.GetContent()
		if err != nil {
			log.Error().
				Str("org", owner).
				Str("repo", repo).
				Str("area", polName).
				Str("path", wfc.GetPath()).
				Str("downloadURL", wfc.GetDownloadURL()).
				Err(err).
				Msg("Unexpected error while getting workflow file content. Skipping.")
			continue
		}
		bc := []byte(sc)
		wf, errs := actionlint.Parse(bc)
		if len(errs) > 0 || wf == nil {
			var errors []error
			for _, err := range errs {
				errors = append(errors, fmt.Errorf("actionlist.Parse error: %w", err))
			}
			log.Warn().
				Str("org", owner).
				Str("repo", repo).
				Str("area", polName).
				Str("path", wfc.GetPath()).
				Errs("errors", errors).
				Msg("Errors while parsing workflow file content.")
		}
		if wf == nil {
			continue
		}
		workflows = append(workflows, &workflowMetadata{
			filename: wfc.GetName(),
			workflow: wf,
		})
	}
	return workflows, nil
}

// listReleasesReal uses the GitHub API to list releases for a repo.
// Docs: https://docs.github.com/en/rest/releases/releases#list-releases
func listReleasesReal(ctx context.Context, c *github.Client, owner, repo string) ([]*github.RepositoryRelease, error) {
	releases, _, err := c.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{})
	if err != nil {
		return nil, err
	}
	return releases, nil
}
