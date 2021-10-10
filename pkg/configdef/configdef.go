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

package configdef

// ActionConfig is used in org/repo-level config to
// define the action configuration.
type ActionConfig struct {
	// IssueLabel : set to override GitHubIssueLabel in operator.go.
	// GitHubIssueLabel is the label used to tag, search, and identify GitHub
	// Issues created by the bot.
	IssueLabel string
}
