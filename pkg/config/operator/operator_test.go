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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// TestDefaults pins the built-in defaults. Allstar previously shipped with the
// OpenSSF-operated instance's App ID and Secret Manager path baked in; those
// are gone, so an operator who configures nothing must get an unusable config
// that Validate rejects, not someone else's app.
func TestDefaults(t *testing.T) {
	if setAppID != 0 {
		t.Errorf("setAppID default = %d, want 0 (no app configured)", setAppID)
	}
	if setKeySecret != "direct" {
		t.Errorf("setKeySecret default = %q, want %q", setKeySecret, "direct")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		Name       string
		AppID      int64
		KeySecret  string
		PrivateKey string
		WantErr    bool
		// WantErrContains is the env var name the message must name, so the
		// operator knows what to set.
		WantErrContains string
	}{
		{
			Name:            "NothingConfigured",
			AppID:           0,
			KeySecret:       "direct",
			WantErr:         true,
			WantErrContains: "APP_ID",
		},
		{
			Name:            "MissingAppID",
			AppID:           0,
			KeySecret:       "awssecretsmanager://allstar-private-key",
			WantErr:         true,
			WantErrContains: "APP_ID",
		},
		{
			Name:            "DirectWithoutPrivateKey",
			AppID:           123,
			KeySecret:       "direct",
			WantErr:         true,
			WantErrContains: "PRIVATE_KEY",
		},
		{
			Name:       "DirectWithPrivateKey",
			AppID:      123,
			KeySecret:  "direct",
			PrivateKey: "fake-private-key",
			WantErr:    false,
		},
		{
			Name:      "RuntimevarSecret",
			AppID:     123,
			KeySecret: "awssecretsmanager://allstar-private-key",
			WantErr:   false,
		},
		{
			Name:      "GCPRuntimevarSecretStillSupported",
			AppID:     123,
			KeySecret: "gcpsecretmanager://projects/my-project/secrets/allstar-private-key?decoder=bytes",
			WantErr:   false,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			AppID = test.AppID
			KeySecret = test.KeySecret
			PrivateKey = test.PrivateKey

			err := Validate()
			if test.WantErr {
				if err == nil {
					t.Fatal("Validate() = nil, want error")
				}
				if !strings.Contains(err.Error(), test.WantErrContains) {
					t.Errorf("Validate() error = %q, want it to name %q",
						err.Error(), test.WantErrContains)
				}
				return
			}
			if err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

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
