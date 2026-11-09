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

// Package testutil provides shared test helpers.
package testutil

import "testing"

// RequireType asserts that value has type T and stops the test if it does not.
func RequireType[T any](tb testing.TB, value any) T {
	tb.Helper()
	result, ok := value.(T)
	if !ok {
		tb.Fatalf("Expected value of type %T, got %T", result, value)
	}
	return result
}
