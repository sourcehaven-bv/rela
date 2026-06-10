package lua

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDateTestRuntime(t *testing.T) (*Runtime, *strings.Builder) {
	t.Helper()
	var sb strings.Builder
	rt := NewReader(ReadDeps{}, &sb)
	return rt, &sb
}

func TestDateAdd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		date   string
		offset string
		want   string
	}{
		{"add days", "2025-01-15", "7d", "2025-01-22"},
		{"add weeks", "2025-01-15", "2w", "2025-01-29"},
		{"add months", "2025-01-15", "1m", "2025-02-15"},
		{"add years", "2025-01-15", "1y", "2026-01-15"},
		{"negative days", "2025-01-15", "-3d", "2025-01-12"},
		{"negative months", "2025-03-15", "-1m", "2025-02-15"},
		{"month overflow jan31 plus 1m", "2025-01-31", "1m", "2025-03-03"},
		{"leap year", "2024-02-29", "1y", "2025-03-01"},
		{"rfc3339 input", "2025-01-15T10:00:00Z", "3d", "2025-01-18"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, buf := newDateTestRuntime(t)
			defer rt.Close()

			script := `rela.output(rela.date_add("` + tt.date + `", "` + tt.offset + `"))`
			err := rt.RunString(script)
			require.NoError(t, err)

			var result string
			require.NoError(t, json.Unmarshal([]byte(buf.String()), &result))
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestDateAddError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		script string
	}{
		{"invalid date", `rela.date_add("not-a-date", "1d")`},
		{"invalid offset", `rela.date_add("2025-01-15", "abc")`},
		{"empty offset", `rela.date_add("2025-01-15", "x")`},
		{"bad unit", `rela.date_add("2025-01-15", "3z")`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, _ := newDateTestRuntime(t)
			defer rt.Close()

			err := rt.RunString(tt.script)
			require.Error(t, err)
		})
	}
}

func TestDateWeekday(t *testing.T) {
	t.Parallel()
	tests := []struct {
		date string
		want string
	}{
		{"2025-01-06", "monday"},
		{"2025-01-07", "tuesday"},
		{"2025-01-08", "wednesday"},
		{"2025-01-09", "thursday"},
		{"2025-01-10", "friday"},
		{"2025-01-11", "saturday"},
		{"2025-01-12", "sunday"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			rt, buf := newDateTestRuntime(t)
			defer rt.Close()

			script := `rela.output(rela.date_weekday("` + tt.date + `"))`
			err := rt.RunString(script)
			require.NoError(t, err)

			var result string
			require.NoError(t, json.Unmarshal([]byte(buf.String()), &result))
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestDateNextWeekday(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		date string
		day  string
		want string
	}{
		{"monday to friday", "2025-01-06", "friday", "2025-01-10"},
		{"friday to monday", "2025-01-10", "monday", "2025-01-13"},
		{"same day advances 7", "2025-01-06", "monday", "2025-01-13"},
		{"saturday to saturday", "2025-01-11", "saturday", "2025-01-18"},
		{"case insensitive", "2025-01-06", "Friday", "2025-01-10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, buf := newDateTestRuntime(t)
			defer rt.Close()

			script := `rela.output(rela.date_next_weekday("` + tt.date + `", "` + tt.day + `"))`
			err := rt.RunString(script)
			require.NoError(t, err)

			var result string
			require.NoError(t, json.Unmarshal([]byte(buf.String()), &result))
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestDateNextWeekdayError(t *testing.T) {
	t.Parallel()
	rt, _ := newDateTestRuntime(t)
	defer rt.Close()

	err := rt.RunString(`rela.date_next_weekday("2025-01-06", "notaday")`)
	require.Error(t, err)
}

func TestRruleNext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		rrule string
		after string
		want  string
	}{
		{
			"weekly saturday",
			"FREQ=WEEKLY;BYDAY=SA;DTSTART=20250101T000000Z",
			"2025-01-06",
			"2025-01-11",
		},
		{
			"monthly first day",
			"FREQ=MONTHLY;BYMONTHDAY=1;DTSTART=20250101T000000Z",
			"2025-01-15",
			"2025-02-01",
		},
		{
			"monthly last day",
			"FREQ=MONTHLY;BYMONTHDAY=-1;DTSTART=20250101T000000Z",
			"2025-01-15",
			"2025-01-31",
		},
		{
			"quarterly first saturday",
			"FREQ=MONTHLY;INTERVAL=3;BYDAY=1SA;DTSTART=20250101T000000Z",
			"2025-01-06",
			"2025-04-05",
		},
		{
			"with RRULE prefix",
			"RRULE:FREQ=WEEKLY;BYDAY=MO;DTSTART=20250101T000000Z",
			"2025-01-07",
			"2025-01-13",
		},
		{
			"every 2 weeks",
			"FREQ=WEEKLY;INTERVAL=2;DTSTART=20250106T000000Z",
			"2025-01-06",
			"2025-01-20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, buf := newDateTestRuntime(t)
			defer rt.Close()

			script := `rela.output(rela.rrule_next("` + tt.rrule + `", "` + tt.after + `"))`
			err := rt.RunString(script)
			require.NoError(t, err)

			var result string
			require.NoError(t, json.Unmarshal([]byte(buf.String()), &result))
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestRruleNextDefaultsToToday(t *testing.T) {
	t.Parallel()
	rt, buf := newDateTestRuntime(t)
	defer rt.Close()

	// Without after date, should use today and return a future date
	script := `rela.output(rela.rrule_next("FREQ=DAILY"))`
	err := rt.RunString(script)
	require.NoError(t, err)

	var result string
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &result))
	assert.NotEmpty(t, result)
	// Result should be a valid date string
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, result)
}

func TestRruleNextExhausted(t *testing.T) {
	t.Parallel()
	rt, buf := newDateTestRuntime(t)
	defer rt.Close()

	// Rule with COUNT=1 and DTSTART in the past — only occurrence is Jan 1,
	// so asking for next after Jan 1 returns nil.
	script := `rela.output(rela.rrule_next("FREQ=DAILY;COUNT=1;DTSTART=20250101T000000Z", "2025-01-01"))`
	err := rt.RunString(script)
	require.NoError(t, err)

	assert.Equal(t, "null", strings.TrimSpace(buf.String()))
}

func TestRruleNextError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		script string
	}{
		{"invalid rrule", `rela.rrule_next("INVALID_RRULE")`},
		{"interval without dtstart", `rela.rrule_next("FREQ=WEEKLY;INTERVAL=2", "2025-01-06")`},
		{"interval 3 without dtstart", `rela.rrule_next("FREQ=MONTHLY;INTERVAL=3", "2025-01-06")`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, _ := newDateTestRuntime(t)
			defer rt.Close()

			err := rt.RunString(tt.script)
			require.Error(t, err)
		})
	}
}

func TestParseOffset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		years  int
		months int
		days   int
	}{
		{"7d", 0, 0, 7},
		{"2w", 0, 0, 14},
		{"3m", 0, 3, 0},
		{"1y", 1, 0, 0},
		{"-5d", 0, 0, -5},
		{"-2m", 0, -2, 0},
		{"10d", 0, 0, 10},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			y, m, d, err := parseOffset(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.years, y)
			assert.Equal(t, tt.months, m)
			assert.Equal(t, tt.days, d)
		})
	}
}

func TestParseOffsetError(t *testing.T) {
	t.Parallel()
	tests := []string{"", "x", "abc", "3z", "d"}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, _, _, err := parseOffset(input)
			require.Error(t, err)
		})
	}
}
