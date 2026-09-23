package tasks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTasksCronUnit(t *testing.T) {
	t.Parallel()

	t.Run("empty expression returns error", func(t *testing.T) {
		t.Parallel()
		cronSchedule, err := ParseCron("   ")
		require.Error(t, err)
		require.Nil(t, cronSchedule)
		require.Contains(t, err.Error(), "empty cron expression")
	})

	t.Run("invalid field counts return error", func(t *testing.T) {
		t.Parallel()
		for _, expr := range []string{"* * * *", "* * * * * *", "hello"} {
			cronSchedule, err := ParseCron(expr)
			require.Error(t, err)
			require.Nil(t, cronSchedule)
			require.Contains(t, err.Error(), "expected 5 fields")
		}
	})

	t.Run("invalid minute field returns error", func(t *testing.T) {
		t.Parallel()
		for _, expr := range []string{"60 * * * *", "-1 * * * *", "abc * * * *", "*/0 * * * *", "5-2 * * * *", "1,,2 * * * *"} {
			cronSchedule, err := ParseCron(expr)
			require.Error(t, err)
			require.Nil(t, cronSchedule)
		}
	})

	t.Run("invalid hour field returns error", func(t *testing.T) {
		t.Parallel()
		cronSchedule, err := ParseCron("0 24 * * *")
		require.Error(t, err)
		require.Nil(t, cronSchedule)
		require.Contains(t, err.Error(), "invalid hour field")
	})

	t.Run("invalid day of month field returns error", func(t *testing.T) {
		t.Parallel()
		cronSchedule, err := ParseCron("0 0 32 * *")
		require.Error(t, err)
		require.Nil(t, cronSchedule)
		require.Contains(t, err.Error(), "invalid day-of-month field")
	})

	t.Run("invalid month field returns error", func(t *testing.T) {
		t.Parallel()
		cronSchedule, err := ParseCron("0 0 1 13 *")
		require.Error(t, err)
		require.Nil(t, cronSchedule)
		require.Contains(t, err.Error(), "invalid month field")
	})

	t.Run("invalid day of week field returns error", func(t *testing.T) {
		t.Parallel()
		cronSchedule, err := ParseCron("0 0 * * 8")
		require.Error(t, err)
		require.Nil(t, cronSchedule)
		require.Contains(t, err.Error(), "invalid day-of-week field")
	})

	t.Run("invalid multi-slash and multi-dash syntax returns error", func(t *testing.T) {
		t.Parallel()
		_, err := ParseCron("*/2/3 * * * *")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid step syntax")

		_, err = ParseCron("1-2-3 * * * *")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid range syntax")
	})

	t.Run("parses all standard cron aliases", func(t *testing.T) {
		t.Parallel()
		aliases := []string{"@hourly", "@daily", "@midnight", "@weekly", "@monthly", "@yearly", "@annually"}
		for _, alias := range aliases {
			cronSchedule, err := ParseCron(alias)
			require.NoError(t, err)
			require.NotNil(t, cronSchedule)
		}
	})

	t.Run("parses step intervals, ranges, and comma lists", func(t *testing.T) {
		t.Parallel()
		cronSchedule, err := ParseCron("*/15 0-4,12 1-15,20 1-6 1-5")
		require.NoError(t, err)
		require.NotNil(t, cronSchedule)

		require.True(t, cronSchedule.Minutes[0])
		require.True(t, cronSchedule.Minutes[15])
		require.True(t, cronSchedule.Minutes[30])
		require.True(t, cronSchedule.Minutes[45])
		require.False(t, cronSchedule.Minutes[10])

		require.True(t, cronSchedule.Hours[0])
		require.True(t, cronSchedule.Hours[4])
		require.True(t, cronSchedule.Hours[12])
		require.False(t, cronSchedule.Hours[5])

		require.True(t, cronSchedule.DaysOfMonth[1])
		require.True(t, cronSchedule.DaysOfMonth[15])
		require.True(t, cronSchedule.DaysOfMonth[20])
		require.False(t, cronSchedule.DaysOfMonth[16])

		require.True(t, cronSchedule.Months[1])
		require.True(t, cronSchedule.Months[6])
		require.False(t, cronSchedule.Months[7])

		require.True(t, cronSchedule.DaysOfWeek[1])  // Monday
		require.True(t, cronSchedule.DaysOfWeek[5])  // Friday
		require.False(t, cronSchedule.DaysOfWeek[0]) // Sunday
	})

	t.Run("parses day of week 7 as sunday (0)", func(t *testing.T) {
		t.Parallel()
		cronSchedule, err := ParseCron("0 0 * * 7")
		require.NoError(t, err)
		require.NotNil(t, cronSchedule)
		require.True(t, cronSchedule.DaysOfWeek[0])
	})

	t.Run("calculates next occurrence in UTC", func(t *testing.T) {
		t.Parallel()
		cronSchedule, err := ParseCron("0 12 * * *")
		require.NoError(t, err)

		baseTime := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
		nextTime := cronSchedule.Next(baseTime, nil)
		expectedTime := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
		require.Equal(t, expectedTime, nextTime)

		// When called after 12:00, advances to next day
		afterTime := time.Date(2026, 9, 23, 12, 30, 0, 0, time.UTC)
		nextDayTime := cronSchedule.Next(afterTime, time.UTC)
		expectedNextDayTime := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
		require.Equal(t, expectedNextDayTime, nextDayTime)
	})

	t.Run("calculates next occurrence with timezone location", func(t *testing.T) {
		t.Parallel()
		cronSchedule, err := ParseCron("0 9 * * *")
		require.NoError(t, err)

		location, err := time.LoadLocation("Asia/Tokyo")
		require.NoError(t, err)

		baseTime := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC) // 09:00 in Tokyo
		// After 09:00 Tokyo time:
		laterTime := baseTime.Add(time.Hour)
		nextTokyoTime := cronSchedule.Next(laterTime, location)

		require.Equal(t, 9, nextTokyoTime.Hour())
		require.Equal(t, 24, nextTokyoTime.Day())
	})

	t.Run("day of month wildcard with specific day of week", func(t *testing.T) {
		t.Parallel()
		// Only run on Fridays at 17:00
		cronSchedule, err := ParseCron("0 17 * * 5")
		require.NoError(t, err)

		// 2026-09-23 is Wednesday
		baseTime := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
		nextTime := cronSchedule.Next(baseTime, time.UTC)
		require.Equal(t, time.Friday, nextTime.Weekday())
		require.Equal(t, 17, nextTime.Hour())
		require.Equal(t, 25, nextTime.Day()) // Friday Sept 25
	})

	t.Run("specific day of month with wildcard day of week", func(t *testing.T) {
		t.Parallel()
		// Run on the 1st of every month at midnight
		cronSchedule, err := ParseCron("0 0 1 * *")
		require.NoError(t, err)

		baseTime := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
		nextTime := cronSchedule.Next(baseTime, time.UTC)
		require.Equal(t, 1, nextTime.Day())
		require.Equal(t, time.October, nextTime.Month())
	})

	t.Run("both day of month and day of week specified uses union", func(t *testing.T) {
		t.Parallel()
		// Run on 1st of month OR on Mondays
		cronSchedule, err := ParseCron("0 0 1 * 1")
		require.NoError(t, err)

		// Wednesday 2026-09-23: next is Monday 2026-09-28 before Oct 1
		baseTime := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
		nextTime := cronSchedule.Next(baseTime, time.UTC)
		require.Equal(t, 28, nextTime.Day())
		require.Equal(t, time.Monday, nextTime.Weekday())
	})

	t.Run("unmatchable schedule within limit returns zero time", func(t *testing.T) {
		t.Parallel()
		// Feb 30 never exists
		cronSchedule, err := ParseCron("0 0 30 2 *")
		require.NoError(t, err)

		baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		nextTime := cronSchedule.Next(baseTime, time.UTC)
		require.True(t, nextTime.IsZero())
	})
}
