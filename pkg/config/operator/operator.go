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

// Package operator contains config to be set by the GitHub App operator
package operator

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// AppID should be set to the application ID of the created GitHub App. See:
// https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#authenticating-as-a-github-app
const setAppID = 119816

var AppID int64

// Raw value of the private key for the App. See:
// https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#generating-a-private-key
var PrivateKey string

// KeySecret should be set to the name of a secret containing a private key for
// the App. See:
// https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#generating-a-private-key
// The secret is retrieved with gocloud.dev/runtimevar.
const setKeySecret = "gcpsecretmanager://projects/allstar-ossf/secrets/allstar-private-key?decoder=bytes"

var KeySecret string

// GitHubEnterpriseUrl allows to configure the usage a GitHub enterprise instance
var GitHubEnterpriseUrl string

// OrgConfigRepo is the name of the expected org-level repo to contain config.
const OrgConfigRepo = ".allstar"

// OrgConfigDir is the name of the expected directory in the org-level .github
// repo.
const OrgConfigDir = "allstar"

// RepoConfigDir is the name of the expected directory in each repo to contain
// repo-level config.
const RepoConfigDir = ".allstar"

// AppConfigFile is the name of the expected file in org or repo level config.
const AppConfigFile = "allstar.yaml"

// DoNothingOnOptOut is a configuration flag indicating if allstar should do
// nothing and skip the corresponding checks when a repository is opted out.
// Can be configured with environment variable DO_NOTHING_ON_OPT_OUT, where
// the value should be a string equivalent of a bool, as accepted by
// strconv.ParseBool.
const setDoNothingOnOptOut = false

var DoNothingOnOptOut bool

// LogLevel is a configuration flag indicating the minimum logging level that
// allstar should use when emitting logs. Can be configured with the environment
// variable ALLSTAR_LOG_LEVEL, which must be one of the following strings:
// panic ; fatal ; error ; warn ; info ; debug ; trace
// If an unparsable string is provided, then allstar will automatically default
// to info level.
const setLogLevel = zerolog.InfoLevel

var LogLevel zerolog.Level

// GitHubIssueLabel is the label used to tag, search, and identify GitHub
// Issues created by the bot.
const GitHubIssueLabel = "allstar"

// GitHubIssueFooter is added to the end of GitHub issues.
const GitHubIssueFooter = `This issue will auto resolve when the policy is in compliance.

Issue created by Allstar. See https://github.com/ossf/allstar/ for more information. For questions specific to the repository, please contact the owner or maintainer.`

// AllowedOrganizations is the set of GitHub repositories on which this Allstar instance
// is allowed to be installed. This allows a public GitHub app to be shared between GitHub
// organizations and repos while restricting installation of the app
var AllowedOrganizations []string

// NoticePingDuration is the duration (in hours) to wait between pinging notice actions,
// such as updating a GitHub issue.
const setNoticePingDurationHrs = (24 * time.Hour)

var NoticePingDuration time.Duration

// NumWorkers is the number of concurrent orginazations/installations the
// Allstar binary will scan concurrently.
const setNumWorkers = 5

var NumWorkers int

var osGetenv func(string) string

func init() {
	osGetenv = os.Getenv
	setVars()
}

func setVars() {
	appIDs := osGetenv("APP_ID")
	appID, err := strconv.ParseInt(appIDs, 10, 64)
	if err == nil {
		AppID = appID
	} else {
		AppID = setAppID
	}

	PrivateKey = osGetenv("PRIVATE_KEY")

	keySecret := osGetenv("KEY_SECRET")
	if keySecret != "" {
		KeySecret = keySecret
	} else {
		KeySecret = setKeySecret
	}

	GitHubEnterpriseUrl = osGetenv("ALLSTAR_GHE_URL")

	doNothingOnOptOutStr := osGetenv("DO_NOTHING_ON_OPT_OUT")
	doNothingOnOptOut, err := strconv.ParseBool(doNothingOnOptOutStr)
	if err == nil {
		DoNothingOnOptOut = doNothingOnOptOut
	} else {
		DoNothingOnOptOut = setDoNothingOnOptOut
	}

	logLevelStr := osGetenv("ALLSTAR_LOG_LEVEL")
	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil || logLevel == zerolog.NoLevel {
		LogLevel = setLogLevel
	} else {
		LogLevel = logLevel
	}
	zerolog.SetGlobalLevel(LogLevel)

	noticePingDurationRaw := osGetenv("NOTICE_PING_DURATION_HOURS")
	noticePingDuration, err := strconv.ParseInt(noticePingDurationRaw, 10, 64)
	if err == nil {
		NoticePingDuration = (time.Duration(noticePingDuration) * time.Hour)
	} else {
		NoticePingDuration = setNoticePingDurationHrs
	}

	allowedOrgs := osGetenv("GITHUB_ALLOWED_ORGS")
	AllowedOrganizations = strings.Split(allowedOrgs, ",")

	nws := osGetenv("ALLSTAR_NUM_WORKERS")
	nw, err := strconv.Atoi(nws)
	if err == nil {
		NumWorkers = nw
	} else {
		NumWorkers = setNumWorkers
	}
}
