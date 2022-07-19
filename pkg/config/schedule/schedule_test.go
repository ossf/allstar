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

package schedule

import (
	"testing"
	"time"

	"github.com/ossf/allstar/pkg/config"
)

func timeFromDay(wd time.Weekday) time.Time {
	return time.Date(1998, 9, 6+int(wd), 11, 0, 0, 0, time.UTC)
}

func setDay(wd time.Weekday) {
	timeNow = func() time.Time {
		return timeFromDay(wd)
	}
}

func TestShouldPerform(t *testing.T) {
	t.Run("AllowDay", func(t *testing.T) {
		sch := &config.ScheduleConfig{
			Days: []string{"saturday", "sunday"},
		}
		setDay(time.Monday)
		if perf := ShouldPerform(sch); perf == false {
			t.Errorf("Expected to perform issue ping")
		}
	})
	t.Run("AllowNilDays", func(t *testing.T) {
		sch := &config.ScheduleConfig{}
		setDay(time.Monday)
		if perf := ShouldPerform(sch); perf == false {
			t.Errorf("Expected to perform issue create")
		}
	})
	t.Run("AllowNilSchedule", func(t *testing.T) {
		var sch *config.ScheduleConfig
		setDay(time.Monday)
		if perf := ShouldPerform(sch); perf == false {
			t.Errorf("Expected to perform issue create")
		}
	})
	t.Run("DenyDay", func(t *testing.T) {
		sch := &config.ScheduleConfig{
			Days: []string{"monday", "tuesday"},
		}
		setDay(time.Tuesday)
		if perf := ShouldPerform(sch); perf == true {
			t.Errorf("Expected to not perform issue ping")
		}
	})
	t.Run("DenyOverride", func(t *testing.T) {
		sch := &config.ScheduleConfig{
			Days: []string{"monday", "wednesday"},
		}
		setDay(time.Wednesday)
		if perf := ShouldPerform(sch); perf == true {
			t.Errorf("Expected to not perform issue ping")
		}
	})
}

func TestMergeSchedules(t *testing.T) {
	sch1 := &config.ScheduleConfig{}
	sch2 := &config.ScheduleConfig{}
	t.Run("Nil", func(t *testing.T) {
		if MergeSchedules(nil, nil, nil) != nil {
			t.Errorf("Expected nil config")
		}
	})
	t.Run("oc", func(t *testing.T) {
		if MergeSchedules(sch1, nil, nil) != sch1 {
			t.Errorf("Expected sch1 config")
		}
	})
	t.Run("orc", func(t *testing.T) {
		if MergeSchedules(nil, sch1, nil) != sch1 {
			t.Errorf("Expected sch1 config")
		}
	})
	t.Run("rc", func(t *testing.T) {
		if MergeSchedules(nil, nil, sch1) != sch1 {
			t.Errorf("Expected sch1 config")
		}
	})
	t.Run("oc-rc", func(t *testing.T) {
		if MergeSchedules(sch1, nil, sch2) != sch2 {
			t.Errorf("Expected sch2 config")
		}
	})
	t.Run("oc-orc", func(t *testing.T) {
		if MergeSchedules(sch1, sch2, nil) != sch2 {
			t.Errorf("Expected sch2 config")
		}
	})
}
