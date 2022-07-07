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

package schedule_test

import (
	"testing"
	"time"

	"github.com/ossf/allstar/pkg/config/schedule"
)

func timeFromDay(weekday time.Weekday) time.Time {
	return time.Date(1998, 9, 6+int(weekday), 11, 0, 0, 0, time.UTC)
}

func TestShouldPerform(t *testing.T) {
	t.Run("AllowDay", func(t *testing.T) {
		sch := &schedule.ScheduleConfig{
			Actions: schedule.ScheduleConfigActions{
				schedule.ScheduleActionIssuePing: false,
			},
			Days: []string{"saturday", "sunday"},
		}
		if perf, err := sch.ShouldPerform(schedule.ScheduleActionIssuePing, timeFromDay(time.Monday)); err != nil || perf == false {
			t.Errorf("Expected to perform issue ping %v", err)
		}
	})
	t.Run("AllowDefault", func(t *testing.T) {
		sch := &schedule.ScheduleConfig{
			Actions: schedule.ScheduleConfigActions{},
			Days:    []string{"monday", "tuesday"},
		}
		if perf, err := sch.ShouldPerform(schedule.ScheduleActionIssueCreate, timeFromDay(time.Monday)); err != nil || perf == false {
			t.Errorf("Expected to perform issue create %v", err)
		}
	})
	t.Run("DenyDay", func(t *testing.T) {
		sch := &schedule.ScheduleConfig{
			Actions: schedule.ScheduleConfigActions{
				schedule.ScheduleActionIssuePing: false,
			},
			Days: []string{"monday", "tuesday"},
		}
		if perf, err := sch.ShouldPerform(schedule.ScheduleActionIssuePing, timeFromDay(time.Tuesday)); err != nil || perf == true {
			t.Errorf("Expected to not perform issue ping %v", err)
		}
	})
	t.Run("DenyOverride", func(t *testing.T) {
		sch := &schedule.ScheduleConfig{
			Actions: schedule.ScheduleConfigActions{
				schedule.ScheduleActionIssueCreate: false,
			},
			Days: []string{"monday", "wednesday"},
		}
		if perf, err := sch.ShouldPerform(schedule.ScheduleActionIssuePing, timeFromDay(time.Wednesday)); err != nil || perf == true {
			t.Errorf("Expected to not perform issue ping %v", err)
		}
	})
}
