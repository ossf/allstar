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

// Package schedule provides the ShouldPerform function for use with
// config.ScheduleConfig.
package schedule

import (
	"strings"
	"time"

	"github.com/ossf/allstar/pkg/config"
	"github.com/rs/zerolog/log"
)

var weekdayStrings = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

var timeNow func() time.Time

func init() {
	timeNow = time.Now
}

// ShouldPerform determines whether an action should be performed based on sch.
// The error may be ignored for default create behavior.
func ShouldPerform(sch *config.ScheduleConfig) bool {
	at := timeNow()
	if sch == nil {
		return true
	}
	// Get the day in timezone specified or default "" => UTC
	loc, err := time.LoadLocation(sch.Timezone)
	if err != nil {
		log.Warn().
			Str("tzstring", sch.Timezone).
			Msg("Failed to load malformed timezone.")
		return true
	}
	weekdayInLoc := at.In(loc).Weekday()
	// Check if weekday match in days
	for i, wds := range sch.Days {
		// Allow up to 3 days to be silenced
		if i > 2 {
			break
		}
		wds = strings.ToLower(wds)
		if wd, ok := weekdayStrings[wds]; ok {
			if wd == weekdayInLoc {
				return false
			}
		}
	}
	return true
}

// MergeSchedules gets the preferred ScheduleConfig from the ScheduleConfigs provided
func MergeSchedules(oc *config.ScheduleConfig, orc, rc *config.ScheduleConfig) *config.ScheduleConfig {
	var mc *config.ScheduleConfig

	for _, cc := range []*config.ScheduleConfig{oc, orc, rc} {
		if cc != nil {
			mc = cc
		}
	}

	return mc
}
