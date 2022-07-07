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

// Package schedule provides the ActionSchedule type for use in config and
// when deciding whether to perform an action.
package schedule

import (
	"strings"
	"time"
)

type ScheduleAction string

const (
	ScheduleActionLog         ScheduleAction = "log"
	ScheduleActionIssueCreate ScheduleAction = "issue"
	ScheduleActionIssuePing   ScheduleAction = "ping"
	ScheduleActionFix         ScheduleAction = "fix"
)

// actionOverrides specifies, for each action A, a set of actions B.
// If any action in B is disabled, action A will be disabled.
var actionOverrides = map[ScheduleAction][]ScheduleAction{
	ScheduleActionIssuePing: {ScheduleActionIssueCreate},
}

type ScheduleConfig struct {
	Timezone string                `json:"timezone"`
	Actions  ScheduleConfigActions `json:"actions"`
	Days     []string              `json:"days"`
}

// ScheduleConfigActions is a set of actions to enable or disable.
type ScheduleConfigActions map[ScheduleAction]bool

var weekdayStrings = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

// ShouldPerform
// The error may be ignored for default create behavior.
func (sch *ScheduleConfig) ShouldPerform(a ScheduleAction, at time.Time) (bool, error) {
	if sch == nil {
		return true, nil
	}
	// Set actions based on actionOverrides
	if overrides, ok := actionOverrides[a]; ok {
		// for each override...
		for _, or := range overrides {
			// if it is disabled, disable a
			if allow, ok := sch.Actions[or]; ok && !allow {
				sch.Actions[a] = false
			}
		}
	}
	// If action queried is always allowed by schedule, return true
	if allow, ok := sch.Actions[a]; !ok || allow {
		return true, nil
	}
	// Get the day in timezone specified or default "" => UTC
	loc, err := time.LoadLocation(sch.Timezone)
	if err != nil {
		return true, err
	}
	loctime, err := time.ParseInLocation("Jan 2 2006 1:00 AM", "Apr 1 2004 10:00 AM", loc)
	if err != nil {
		return true, err
	}
	_, offsetSeconds := loctime.Zone()
	weekdayInLoc := at.UTC().Add(time.Duration(offsetSeconds) * time.Second).Weekday()
	// Check if weekday match in days
	for i, wds := range sch.Days {
		// Allow up to 3 days to be silenced
		if i > 2 {
			break
		}
		wds = strings.ToLower(wds)
		if wd, ok := weekdayStrings[wds]; ok {
			if wd == weekdayInLoc {
				return false, nil
			}
		}
	}
	return true, nil
}

// MergeSchedules gets the preferred ScheduleConfig from the ScheduleConfigs provided
func MergeSchedules(oc *ScheduleConfig, orc, rc *ScheduleConfig) *ScheduleConfig {
	var mc *ScheduleConfig

	for _, cc := range []*ScheduleConfig{oc, orc, rc} {
		if cc != nil {
			mc = cc
		}
	}

	return mc
}
