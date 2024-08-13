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
		GitHubEnterpriseUrl   string
		NoticePingDurationHrs string
		PrivateKey            string
		DoNothingOnOptOut     string
		OperatorAllowlist     string
		ExpAppID              int64
		ExpKeySecret          string
		ExpDoNothingOnOptOut  bool
		ExpPrivateKey         string
		ExpNoticePingDuration time.Duration
		ExpOperatorAllowlist  []string
	}{
		{
			Name:                  "NoVars",
			AppID:                 "",
			KeySecret:             "",
			GitHubEnterpriseUrl:   "",
			DoNothingOnOptOut:     "",
			ExpAppID:              setAppID,
			ExpKeySecret:          setKeySecret,
			ExpDoNothingOnOptOut:  setDoNothingOnOptOut,
			ExpNoticePingDuration: (24 * time.Hour),
			ExpOperatorAllowlist:  []string{""},
		},
		{
			Name:                  "SetVars",
			AppID:                 "123",
			KeySecret:             "asdf",
			GitHubEnterpriseUrl:   "https://ghe.example.com",
			DoNothingOnOptOut:     "true",
			ExpAppID:              123,
			ExpKeySecret:          "asdf",
			ExpDoNothingOnOptOut:  true,
			ExpNoticePingDuration: (24 * time.Hour),
			ExpOperatorAllowlist:  []string{""},
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
			ExpOperatorAllowlist:  []string{""},
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
			ExpOperatorAllowlist:  []string{""},
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
			ExpOperatorAllowlist:  []string{""},
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
			ExpOperatorAllowlist:  []string{""},
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
			ExpOperatorAllowlist:  []string{""},
		},
		{
			Name:                  "EmptyAllowlist",
			AppID:                 "",
			KeySecret:             "",
			DoNothingOnOptOut:     "",
			NoticePingDurationHrs: "",
			ExpAppID:              setAppID,
			ExpKeySecret:          setKeySecret,
			ExpDoNothingOnOptOut:  setDoNothingOnOptOut,
			ExpNoticePingDuration: (24 * time.Hour),
			OperatorAllowlist:     "",
			ExpOperatorAllowlist:  []string{""},
		},
		{
			Name:                  "AllowlistTrailingComma",
			AppID:                 "",
			KeySecret:             "",
			DoNothingOnOptOut:     "",
			NoticePingDurationHrs: "",
			ExpAppID:              setAppID,
			ExpKeySecret:          setKeySecret,
			ExpDoNothingOnOptOut:  setDoNothingOnOptOut,
			ExpNoticePingDuration: (24 * time.Hour),
			OperatorAllowlist:     "org-1,",
			ExpOperatorAllowlist:  []string{"org-1", ""},
		},
		{
			Name:                  "Allowlist",
			AppID:                 "",
			KeySecret:             "",
			DoNothingOnOptOut:     "",
			NoticePingDurationHrs: "",
			ExpAppID:              setAppID,
			ExpKeySecret:          setKeySecret,
			ExpDoNothingOnOptOut:  setDoNothingOnOptOut,
			ExpNoticePingDuration: (24 * time.Hour),
			OperatorAllowlist:     "org-1,org-2,org-3",
			ExpOperatorAllowlist:  []string{"org-1", "org-2", "org-3"},
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
				if in == "ALLSTAR_GHE_URL" {
					return test.GitHubEnterpriseUrl
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
				if in == "GITHUB_ALLOWED_ORGS" {
					return test.OperatorAllowlist
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
			if diff := cmp.Diff(test.GitHubEnterpriseUrl, GitHubEnterpriseUrl); diff != "" {
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
			if diff := cmp.Diff(test.ExpOperatorAllowlist, AllowedOrganizations); diff != "" {
				t.Errorf("Unexpected results. (-want +got):\n%s", diff)
			}
		})
	}
}
