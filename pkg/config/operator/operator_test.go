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

// Package operator contains config to be set by the GitHub App operator
package operator

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSetVars(t *testing.T) {
	tests := []struct {
		Name         string
		AppID        string
		KeySecret    string
		ExpAppID     int64
		ExpKeySecret string
	}{
		{
			Name:         "NoVars",
			AppID:        "",
			KeySecret:    "",
			ExpAppID:     setAppID,
			ExpKeySecret: setKeySecret,
		},
		{
			Name:         "SetVars",
			AppID:        "123",
			KeySecret:    "asdf",
			ExpAppID:     123,
			ExpKeySecret: "asdf",
		},
		{
			Name:         "BadInt",
			AppID:        "notint",
			KeySecret:    "",
			ExpAppID:     setAppID,
			ExpKeySecret: setKeySecret,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			osGetenv = func(in string) string {
				if in == "APP_ID" {
					return test.AppID
				}
				if in == "KEY_SECRET" {
					return test.KeySecret
				}
				return ""
			}
			setVars()
			if diff := cmp.Diff(test.ExpAppID, AppID); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.ExpKeySecret, KeySecret); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}
