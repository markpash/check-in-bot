package checkinbot

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type cronSchedule struct {
	minutes map[int]bool
	hours   map[int]bool
	dom     map[int]bool
	months  map[int]bool
	dow     map[int]bool
}

func parseCronSchedule(expr string) (cronSchedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return cronSchedule{}, fmt.Errorf("cron schedule must have 5 fields: minute hour day-of-month month day-of-week")
	}

	minutes, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("invalid minute field: %w", err)
	}
	hours, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("invalid hour field: %w", err)
	}
	dom, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("invalid day-of-month field: %w", err)
	}
	months, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("invalid month field: %w", err)
	}
	dow, err := parseCronField(fields[4], 0, 7)
	if err != nil {
		return cronSchedule{}, fmt.Errorf("invalid day-of-week field: %w", err)
	}
	if dow[7] {
		dow[0] = true
		delete(dow, 7)
	}

	return cronSchedule{
		minutes: minutes,
		hours:   hours,
		dom:     dom,
		months:  months,
		dow:     dow,
	}, nil
}

func parseCronField(field string, min, max int) (map[int]bool, error) {
	values := map[int]bool{}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty field segment")
		}
		segment, err := expandCronPart(part, min, max)
		if err != nil {
			return nil, err
		}
		for _, value := range segment {
			values[value] = true
		}
	}
	return values, nil
}

func expandCronPart(part string, min, max int) ([]int, error) {
	base := part
	step := 1

	if strings.Contains(part, "/") {
		pieces := strings.Split(part, "/")
		if len(pieces) != 2 {
			return nil, fmt.Errorf("invalid step syntax %q", part)
		}
		base = pieces[0]
		n, err := strconv.Atoi(pieces[1])
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid step %q", pieces[1])
		}
		step = n
	}

	rangeMin := min
	rangeMax := max
	if base != "*" {
		if strings.Contains(base, "-") {
			bounds := strings.Split(base, "-")
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid range %q", base)
			}
			start, err := strconv.Atoi(bounds[0])
			if err != nil {
				return nil, fmt.Errorf("invalid range start %q", bounds[0])
			}
			end, err := strconv.Atoi(bounds[1])
			if err != nil {
				return nil, fmt.Errorf("invalid range end %q", bounds[1])
			}
			rangeMin = start
			rangeMax = end
		} else {
			value, err := strconv.Atoi(base)
			if err != nil {
				return nil, fmt.Errorf("invalid value %q", base)
			}
			rangeMin = value
			rangeMax = value
		}
	}

	if rangeMin < min || rangeMax > max || rangeMin > rangeMax {
		return nil, fmt.Errorf("value %d-%d out of range %d-%d", rangeMin, rangeMax, min, max)
	}

	values := make([]int, 0, ((rangeMax-rangeMin)/step)+1)
	for value := rangeMin; value <= rangeMax; value += step {
		values = append(values, value)
	}
	return values, nil
}

func (s cronSchedule) matches(t time.Time) bool {
	return s.minutes[t.Minute()] &&
		s.hours[t.Hour()] &&
		s.dom[t.Day()] &&
		s.months[int(t.Month())] &&
		s.dow[int(t.Weekday())]
}

func nextCronTime(expr string, now time.Time) (time.Time, error) {
	schedule, err := parseCronSchedule(expr)
	if err != nil {
		return time.Time{}, err
	}

	candidate := now.UTC().Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 5*366*24*60; i++ {
		if schedule.matches(candidate) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("cron schedule %q did not match any time in search window", expr)
}

func parseDBTime(value string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", value)
}
