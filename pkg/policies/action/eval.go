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
	"github.com/google/go-github/v43/github"
)

var requireWorkflowOnForRequire = []string{"pull_request", "push"}

// evaluateActionDenied evaluates an Action against a set of Rules
func evaluateActionDenied(ctx context.Context, c *github.Client, rules []*internalRule, action *actionMetadata, gc globCache, sc semverCache) (*denyRuleEvaluationResult, []error) {
	result := &denyRuleEvaluationResult{
		denied:         false,
		actionMetadata: action,
	}

	var errs []error

	for _, r := range rules {
		stepResult := &denyRuleEvaluationStepResult{
			status: denyRuleStepStatusError,
			rule:   r,
		}
		switch r.Method {
		case "allow":
			fallthrough
		case "require":
			// Check if Action contained within allow or require
			if r.Actions == nil {
				// All Actions allowed by this step
				stepResult.status = denyRuleStepStatusAllowed
				break
			}
			for _, a := range r.Actions {
				match, matchName, _, err := a.match(ctx, c, action, gc, sc)
				if err != nil {
					errs = append(errs, err)
					stepResult.status = denyRuleStepStatusError
					continue
				}
				if !match {
					if matchName {
						stepResult.status = denyRuleStepStatusActionVersionMismatch
						stepResult.ruleVersionConstraint = a.Version
						continue
					}
					stepResult.status = denyRuleStepStatusMissingAction
					continue
				}
				// This is a permissible Action
				stepResult.status = denyRuleStepStatusAllowed
				break
			}
		case "deny":
			// Check if Action is denied
			if r.Actions == nil {
				stepResult.status = denyRuleStepStatusDenied
				result.denied = true
				result.denyingRule = r
				break
			}
			for _, a := range r.Actions {
				match, _, _, err := a.match(ctx, c, action, gc, sc)
				if err != nil {
					errs = append(errs, err)
					stepResult.status = denyRuleStepStatusError
					break
				}
				if !match {
					stepResult.status = denyRuleStepStatusMissingAction
					continue
				}
				// This is a denied Action
				stepResult.status = denyRuleStepStatusDenied
				result.denied = true
				result.denyingRule = r
				break
			}
		default:
			continue
		}
		result.steps = append(result.steps, stepResult)
		if len(result.steps) > 0 {
			// Exit if previous step has specifically allowed or denied the Action.
			lastStatus := result.steps[len(result.steps)-1].status
			if lastStatus == denyRuleStepStatusAllowed || lastStatus == denyRuleStepStatusDenied {
				break
			}
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
		satisfied: false,

		numberRequired:  useCount,
		numberSatisfied: 0,

		rule: rule,
	}

	for _, ra := range rule.Actions {
		// Check if this rule is satisfied

		satisfied := false
		var suggestedFix *requireRuleEvaluationFix

		for _, a := range actions {
			match, matchName, _, err := ra.match(ctx, c, a, gc, sc)
			if err != nil {
				return nil, err
			}
			if !match {
				if matchName {
					// Version mismatch
					suggestedFix = &requireRuleEvaluationFix{
						fixMethod:               requireRuleEvaluationFixMethodUpdate,
						actionName:              a.name,
						actionVersionConstraint: ra.Version,
					}
					break
				}
				// Name mismatch, keep looking
				continue
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
				suggestedFix = &requireRuleEvaluationFix{
					fixMethod:               requireRuleEvaluationFixMethodEnable,
					actionName:              a.name,
					actionVersionConstraint: ra.Version,
					workflowName:            a.workflowName,
				}
				break
			}

			// Check if passing (if the Action is required to be)

			if rule.MustPass {
				passing := false
				runs, err := listWorkflowRunsByFilename(ctx, c, owner, repo, a.workflowFilename)
				if err != nil {
					return nil, err
				}
				for _, run := range runs {
					if run.HeadSHA == nil || *run.HeadSHA != headSHA {
						// Irrelevant run
						continue
					}
					if run.Status != nil && *run.Status == "completed" {
						passing = true
					}
				}
				if !passing {
					// Not passing. Suggest fix.
					if suggestedFix == nil {
						suggestedFix = &requireRuleEvaluationFix{
							fixMethod:               requireRuleEvaluationFixMethodFix,
							actionName:              a.name,
							actionVersionConstraint: ra.Version,
						}
					}
					break
				}
			}

			// Satisfied!
			satisfied = true
			break
		}

		if satisfied {
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

	if result.numberSatisfied == result.numberRequired {
		result.satisfied = true
	}

	return result, nil
}
