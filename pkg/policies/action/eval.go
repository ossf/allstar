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

	"github.com/google/go-github/v59/github"
)

var requireWorkflowOnForRequire = []string{"pull_request", "push"}

// runInProgressStatuses is the set of workflow run statuses that are
// acceptable when mustPass is true on a require rule and the conclusion is
// not success.
var runInProgressStatuses = []string{"in_progress", "queued", "waiting", "requested"}

// evaluateActionDenied evaluates an Action against a set of Rules
func evaluateActionDenied(ctx context.Context, c *github.Client, rules []*internalRule, action *actionMetadata,
	gc globCache, sc semverCache) (*denyRuleEvaluationResult, []error) {
	result := &denyRuleEvaluationResult{
		denied:         false,
		actionMetadata: action,
	}

	var errs []error

	for _, r := range rules {
		// Check if the Action is matched by the rule's ActionSelectors
		ruleMatch := false
		errored := false

		var ruleVersionConstraint string
		versionMismatch := false

		for _, a := range r.Actions {
			match, matchName, _, err := a.match(ctx, c, action, gc, sc)
			if err != nil {
				errs = append(errs, err)
				errored = true
				break
			}
			if !match {
				if matchName {
					versionMismatch = true
					ruleVersionConstraint = a.Version
					continue
				}
				continue
			}
			// This is a denied Action
			ruleMatch = true
			break
		}
		if r.Actions == nil {
			// All Actions qualified for this step
			ruleMatch = true
		}
		stepResult := &denyRuleEvaluationStepResult{
			status:                denyRuleStepStatusError,
			rule:                  r,
			ruleVersionConstraint: "",
		}
		if errored {
			result.steps = append(result.steps, stepResult)
			continue
		}
		switch r.Method {
		case "allow":
			fallthrough
		case "require":
			if ruleMatch {
				stepResult.status = denyRuleStepStatusAllowed
				break
			}
			if versionMismatch {
				stepResult.status = denyRuleStepStatusActionVersionMismatch
				stepResult.ruleVersionConstraint = ruleVersionConstraint
				break
			}
			stepResult.status = denyRuleStepStatusMissingAction
		case "deny":
			if ruleMatch {
				stepResult.status = denyRuleStepStatusDenied
				result.denied = true
				result.denyingRule = r
				break
			}
			if versionMismatch {
				stepResult.status = denyRuleStepStatusActionVersionMismatch
				stepResult.ruleVersionConstraint = ruleVersionConstraint
				break
			}
			stepResult.status = denyRuleStepStatusMissingAction
		}

		result.steps = append(result.steps, stepResult)

		// Exit if previous step has specifically allowed or denied the Action.
		lastStatus := stepResult.status
		if lastStatus == denyRuleStepStatusAllowed || lastStatus == denyRuleStepStatusDenied {
			break
		}
	}

	return result, errs
}

// evaluateRequireRule evaluates a require rule against a set of Actions
func evaluateRequireRule(ctx context.Context, c *github.Client, owner, repo string, rule *internalRule,
	actions []*actionMetadata, headSHA string, gc globCache, sc semverCache) (*requireRuleEvaluationResult, error) {
	if rule.Method != "require" {
		return nil, fmt.Errorf("rule is not a require rule")
	}
	useCount := 1
	if rule.RequireAll {
		useCount = len(rule.Actions)
	}

	result := &requireRuleEvaluationResult{
		numberRequired:  useCount,
		numberSatisfied: 0,

		rule: rule,

		fixes: nil,
	}

	for _, ra := range rule.Actions {
		// Check if this ActionSelector is satisfied

		actionSelectorSatisfied := false
		var suggestedFix *requireRuleEvaluationFix

		// Find Action matching selector ra
		for _, a := range actions {
			match, fixMethod, err := requireActionDetermineFix(ctx, c, owner, repo, ra, a, rule.MustPass, headSHA, gc, sc)

			if err != nil {
				return nil, err
			}

			if match {
				actionSelectorSatisfied = true
				break
			}

			// Not matching

			// Break if this fix is final (eg. match was close enough)
			if fixMethod != requireRuleEvaluationFixMethodAdd {
				suggestedFix = &requireRuleEvaluationFix{
					fixMethod:               fixMethod,
					workflowName:            a.workflowName,
					actionName:              ra.Name,
					actionVersionConstraint: ra.Version,
				}
				break
			}
		}

		if actionSelectorSatisfied {
			result.numberSatisfied++
			continue
		}

		// Not passing due to missing Action, add "add" fix suggestion

		if suggestedFix == nil {
			suggestedFix = &requireRuleEvaluationFix{
				fixMethod:               requireRuleEvaluationFixMethodAdd,
				actionName:              ra.Name,
				actionVersionConstraint: ra.Version,
			}
		}

		result.fixes = append(result.fixes, suggestedFix)
	}

	return result, nil
}

// requireActionDetermineFix determines whether an actionMetadata matches
// an ActionSelector and, if not, provides a fix method.
//
// Note that:
//   - if the actionMetadata doesn't have the same Action name as the
//     selector, the returned fix will be requireRuleEvaluationFixMethodAdd.
//   - on error, the match bool is false AND fix method will not be usable.
//   - on match true, the fix method is not to be used.
func requireActionDetermineFix(ctx context.Context, c *github.Client, owner, repo string, ra *ActionSelector, a *actionMetadata,
	mustPass bool, headSHA string, gc globCache, sc semverCache) (match bool, fix requireRuleEvaluationFixMethod, err error) {
	match, matchName, _, err := ra.match(ctx, c, a, gc, sc)
	if err != nil {
		return false, 0, err
	}
	if !match {
		if matchName {
			// Version mismatch
			return false, requireRuleEvaluationFixMethodUpdate, nil
		}
		// Name mismatch, keep looking
		return false, requireRuleEvaluationFixMethodAdd, nil
	}

	on := map[string]struct{}{}
	for _, o := range a.workflowOn {
		on[o.EventName()] = struct{}{}
	}
	hasRequired := true
	for _, requireOn := range requireWorkflowOnForRequire {
		if _, ok := on[requireOn]; !ok {
			hasRequired = false
		}
	}
	if !hasRequired {
		// Workflow does not have required "on" values
		return false, requireRuleEvaluationFixMethodEnable, nil
	}

	if !mustPass {
		// This action matches and is not required to pass
		return true, 0, nil
	}

	// Check if passing (if the Action is required to be)
	runs, err := listWorkflowRunsByFilename(ctx, c, owner, repo, a.workflowFilename)
	if err != nil {
		return false, 0, err
	}
	for _, run := range runs {
		if run.GetHeadSHA() != headSHA {
			// Irrelevant run
			continue
		}
		inProgress := false
		for _, s := range runInProgressStatuses {
			if run.GetStatus() == s {
				inProgress = true
			}
		}
		if inProgress {
			// The check run isn't complete, so OK for now
			return true, 0, nil
		}
		if run.GetConclusion() == "success" {
			// The run is completed and passing!
			return true, 0, nil
		}
		// Not passing and this was the matching commit
		break
	}
	// Not passing. Suggest fix.
	return false, requireRuleEvaluationFixMethodFix, nil
}
