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
	"github.com/ossf/allstar/pkg/policies/binary"
	"github.com/ossf/allstar/pkg/policies/branch"
	"github.com/ossf/allstar/pkg/policies/outside"
	"github.com/ossf/allstar/pkg/policies/security"
	"github.com/ossf/allstar/pkg/policies/workflow"
	"github.com/ossf/allstar/pkg/policydef"
)

// GetPolicies returns a slice of all policies in Allstar.
func GetPolicies() []policydef.Policy {
	return []policydef.Policy{
		binary.NewBinary(),
		branch.NewBranch(),
		outside.NewOutside(),
		security.NewSecurity(),
		workflow.NewWorkflow(),
	}
}
