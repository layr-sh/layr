// Package tasks provides distributed cron and background task orchestration.
package tasks

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	expectedCronFieldCount = 5
	maxCronMinute          = 59
	maxCronHour            = 23
	maxCronDayOfMonth      = 31
	maxCronMonth           = 12
	sundayAlternate        = 7
	maxYearsLookup         = 5
)

// Standard cron aliases mapped to 5-field expressions.
var cronAliases = map[string]string{
	"@hourly":   "0 * * * *",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@weekly":   "0 0 * * 0",
	"@monthly":  "0 0 1 * *",
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
}

// CronSchedule represents a compiled 5-field cron schedule.
type CronSchedule struct {
	OriginalExpression string
	Minutes            map[int]bool
	Hours              map[int]bool
	DaysOfMonth        map[int]bool
	Months             map[int]bool
	DaysOfWeek         map[int]bool
	DayOfMonthWildcard bool
	DayOfWeekWildcard  bool
}

// ParseCron parses and compiles a standard 5-field cron expression or shortcut alias.
func ParseCron(expression string) (*CronSchedule, error) {
	trimmed := strings.TrimSpace(expression)
	if trimmed == "" {
		return nil, fmt.Errorf("empty cron expression")
	}

	if aliasExpr, ok := cronAliases[strings.ToLower(trimmed)]; ok {
		trimmed = aliasExpr
	}

	fields := strings.Fields(trimmed)
	if len(fields) != expectedCronFieldCount {
		return nil, fmt.Errorf("invalid cron expression: expected %d fields, got %d", expectedCronFieldCount, len(fields))
	}

	minutes, err := parseCronField(fields[0], 0, maxCronMinute)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field: %w", err)
	}

	hours, err := parseCronField(fields[1], 0, maxCronHour)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field: %w", err)
	}

	daysOfMonth, err := parseCronField(fields[2], 1, maxCronDayOfMonth)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-month field: %w", err)
	}

	months, err := parseCronField(fields[3], 1, maxCronMonth)
	if err != nil {
		return nil, fmt.Errorf("invalid month field: %w", err)
	}

	daysOfWeek, err := parseDayOfWeekField(fields[4])
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-week field: %w", err)
	}

	return &CronSchedule{
		OriginalExpression: expression,
		Minutes:            minutes,
		Hours:              hours,
		DaysOfMonth:        daysOfMonth,
		Months:             months,
		DaysOfWeek:         daysOfWeek,
		DayOfMonthWildcard: fields[2] == "*",
		DayOfWeekWildcard:  fields[4] == "*",
	}, nil
}

func parseCronField(field string, minLimit, maxLimit int) (map[int]bool, error) {
	result := make(map[int]bool)
	parts := strings.Split(field, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty part in field '%s'", field)
		}

		stepInterval := 1
		rangeSegment := part
		if strings.Contains(part, "/") {
			subParts := strings.Split(part, "/")
			if len(subParts) != 2 {
				return nil, fmt.Errorf("invalid step syntax in '%s'", part)
			}
			parsedStep, stepErr := strconv.Atoi(subParts[1])
			if stepErr != nil || parsedStep <= 0 {
				return nil, fmt.Errorf("invalid step value '%s'", subParts[1])
			}
			stepInterval = parsedStep
			rangeSegment = subParts[0]
		}

		startBound := minLimit
		endBound := maxLimit

		if rangeSegment == "*" {
			// standard wildcard with optional step
		} else if strings.Contains(rangeSegment, "-") {
			rangeParts := strings.Split(rangeSegment, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range syntax in '%s'", rangeSegment)
			}
			rangeStart, startErr := strconv.Atoi(rangeParts[0])
			rangeEnd, endErr := strconv.Atoi(rangeParts[1])
			if startErr != nil || endErr != nil || rangeStart > rangeEnd || rangeStart < minLimit || rangeEnd > maxLimit {
				return nil, fmt.Errorf("invalid range '%s'", rangeSegment)
			}
			startBound = rangeStart
			endBound = rangeEnd
		} else {
			exactNumber, numberErr := strconv.Atoi(rangeSegment)
			if numberErr != nil || exactNumber < minLimit || exactNumber > maxLimit {
				return nil, fmt.Errorf("invalid value '%s'", rangeSegment)
			}
			startBound = exactNumber
			endBound = exactNumber
		}

		for itemIndex := startBound; itemIndex <= endBound; itemIndex += stepInterval {
			result[itemIndex] = true
		}
	}

	return result, nil
}

func parseDayOfWeekField(field string) (map[int]bool, error) {
	rawMatches, err := parseCronField(field, 0, sundayAlternate)
	if err != nil {
		return nil, err
	}

	result := make(map[int]bool)
	for dayIndex := range rawMatches {
		if dayIndex == sundayAlternate {
			result[0] = true // 7 is Sunday, same as 0
		} else {
			result[dayIndex] = true
		}
	}
	return result, nil
}

// Next calculates the next occurrence after the given time in the specified timezone location.
func (schedule *CronSchedule) Next(startTime time.Time, location *time.Location) time.Time {
	if location == nil {
		location = time.UTC
	}

	// Begin checking starting at the next minute boundary
	candidateTime := startTime.In(location).Truncate(time.Minute).Add(time.Minute)
	maxCandidateTime := candidateTime.AddDate(maxYearsLookup, 0, 0) // Guard against infinite loop

	for candidateTime.Before(maxCandidateTime) {
		// 1. Month check
		month := int(candidateTime.Month())
		if !schedule.Months[month] {
			candidateTime = time.Date(candidateTime.Year(), candidateTime.Month()+1, 1, 0, 0, 0, 0, location)
			continue
		}

		// 2. Day check (day of month & day of week)
		dayOfMonth := candidateTime.Day()
		dayOfWeek := int(candidateTime.Weekday())

		dayMatch := false
		if schedule.DayOfMonthWildcard && schedule.DayOfWeekWildcard {
			dayMatch = true
		} else if !schedule.DayOfMonthWildcard && !schedule.DayOfWeekWildcard {
			// POSIX: when both are specified, either matching satisfies the schedule
			dayMatch = schedule.DaysOfMonth[dayOfMonth] || schedule.DaysOfWeek[dayOfWeek]
		} else if !schedule.DayOfMonthWildcard {
			dayMatch = schedule.DaysOfMonth[dayOfMonth]
		} else {
			dayMatch = schedule.DaysOfWeek[dayOfWeek]
		}

		if !dayMatch {
			candidateTime = time.Date(candidateTime.Year(), candidateTime.Month(), candidateTime.Day()+1, 0, 0, 0, 0, location)
			continue
		}

		// 3. Hour check
		hour := candidateTime.Hour()
		if !schedule.Hours[hour] {
			candidateTime = time.Date(candidateTime.Year(), candidateTime.Month(), candidateTime.Day(), candidateTime.Hour()+1, 0, 0, 0, location)
			continue
		}

		// 4. Minute check
		minute := candidateTime.Minute()
		if !schedule.Minutes[minute] {
			candidateTime = candidateTime.Add(time.Minute)
			continue
		}

		return candidateTime
	}

	return time.Time{}
}
