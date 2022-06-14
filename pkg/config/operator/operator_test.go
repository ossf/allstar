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
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestSetVars(t *testing.T) {
	tests := []struct {
		Name                  string
		AppID                 string
		KeySecret             string
		NoticePingDurationHrs string
		PrivateKey            string
		DoNothingOnOptOut     string
		ExpAppID              int64
		ExpKeySecret          string
		ExpDoNothingOnOptOut  bool
		ExpPrivateKey         string
		ExpNoticePingDuration time.Duration
	}{
		{
			Name:                  "NoVars",
			AppID:                 "",
			KeySecret:             "",
			DoNothingOnOptOut:     "",
			ExpAppID:              setAppID,
			ExpKeySecret:          setKeySecret,
			ExpDoNothingOnOptOut:  setDoNothingOnOptOut,
			ExpNoticePingDuration: (24 * time.Hour),
		},
		{
			Name:                  "SetVars",
			AppID:                 "123",
			KeySecret:             "asdf",
			DoNothingOnOptOut:     "true",
			ExpAppID:              123,
			ExpKeySecret:          "asdf",
			ExpDoNothingOnOptOut:  true,
			ExpNoticePingDuration: (24 * time.Hour),
		},
		{
			Name:                  "BadInt",
			AppID:                 "notint",
			KeySecret:             "",
			DoNothingOnOptOut:     "",
			ExpAppID:              setAppID,
			ExpKeySecret:          setKeySecret,
			ExpDoNothingOnOptOut:  setDoNothingOnOptOut,
			ExpNoticePingDuration: (24 * time.Hour),
		},
		{
			Name:                  "BadBool",
			AppID:                 "",
			KeySecret:             "",
			DoNothingOnOptOut:     "not-bool",
			ExpAppID:              setAppID,
			ExpKeySecret:          setKeySecret,
			ExpDoNothingOnOptOut:  setDoNothingOnOptOut,
			ExpNoticePingDuration: (24 * time.Hour),
		},
		{
			Name:                  "HasPrivateKey",
			AppID:                 "",
			KeySecret:             "",
			PrivateKey:            "fake-private-key",
			DoNothingOnOptOut:     "",
			ExpAppID:              setAppID,
			ExpKeySecret:          setKeySecret,
			ExpDoNothingOnOptOut:  setDoNothingOnOptOut,
			ExpPrivateKey:         "fake-private-key",
			ExpNoticePingDuration: (24 * time.Hour),
		},
		{
			Name:                  "SetNoticePingDuration",
			AppID:                 "",
			KeySecret:             "",
			DoNothingOnOptOut:     "",
			NoticePingDurationHrs: "48",
			ExpAppID:              setAppID,
			ExpKeySecret:          setKeySecret,
			ExpDoNothingOnOptOut:  setDoNothingOnOptOut,
			ExpNoticePingDuration: (48 * time.Hour),
		},
		{
			Name:                  "HasPrivateKey",
			AppID:                 "",
			KeySecret:             "",
			PrivateKey:            "fake-private-key",
			DoNothingOnOptOut:     "",
			ExpAppID:              setAppID,
			ExpKeySecret:          setKeySecret,
			ExpDoNothingOnOptOut:  setDoNothingOnOptOut,
			ExpPrivateKey:         "fake-private-key",
			ExpNoticePingDuration: (24 * time.Hour),
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
				if in == "DO_NOTHING_ON_OPT_OUT" {
					return test.DoNothingOnOptOut
				}
				if in == "NOTICE_PING_DURATION_HOURS" {
					return test.NoticePingDurationHrs
				}
				if in == "PRIVATE_KEY" {
					return test.PrivateKey
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
			if diff := cmp.Diff(test.ExpDoNothingOnOptOut, DoNothingOnOptOut); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.ExpNoticePingDuration, NoticePingDuration); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.PrivateKey, PrivateKey); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}
