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

// Package policies is used to iterate through the available policies in
// Allstar.
package policies

import (
	"github.com/contentful/allstar/pkg/policies/action"
	"github.com/contentful/allstar/pkg/policies/admin"
	"github.com/contentful/allstar/pkg/policies/binary"
	"github.com/contentful/allstar/pkg/policies/branch"
	"github.com/contentful/allstar/pkg/policies/catalog"
	"github.com/contentful/allstar/pkg/policies/codeowners"
	"github.com/contentful/allstar/pkg/policies/outside"
	"github.com/contentful/allstar/pkg/policies/scorecard"
	"github.com/contentful/allstar/pkg/policies/security"
	"github.com/contentful/allstar/pkg/policies/workflow"
	"github.com/contentful/allstar/pkg/policydef"
)

// GetPolicies returns a slice of all policies in Allstar.
func GetPolicies() []policydef.Policy {
	return []policydef.Policy{
		binary.NewBinary(),
		branch.NewBranch(),
		codeowners.NewCodeowners(),
		outside.NewOutside(),
		scorecard.NewScorecard(),
		security.NewSecurity(),
		workflow.NewWorkflow(),
		action.NewAction(),
		admin.NewAdmin(),
		catalog.NewCatalog(),
	}
}
