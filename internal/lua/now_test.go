package lua

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestClockBindings pins rela.today, rela.now and rela.now_unix to one clock
// reading (TKT-8ZO3W07). rela.now is UTC; rela.today keeps the clock's own
// zone, so the two dates differ just after local midnight.
func TestClockBindings(t *testing.T) {
	t.Parallel()
	cet := time.FixedZone("CET", 3600)
	tests := []struct {
		name      string
		at        time.Time
		wantToday string
		wantNow   string
		wantPlus  string // result of date_add on rela.now plus one day
	}{
		{
			name:      "utc afternoon",
			at:        time.Date(2026, 11, 6, 14, 30, 15, 999, time.UTC),
			wantToday: "2026-11-06",
			wantNow:   "2026-11-06T14:30:15Z",
			wantPlus:  "2026-11-07",
		},
		{
			name:      "just after local midnight",
			at:        time.Date(2026, 11, 7, 0, 30, 0, 0, cet),
			wantToday: "2026-11-07",
			wantNow:   "2026-11-06T23:30:00Z",
			wantPlus:  "2026-11-07",
		},
		{
			name:      "year end",
			at:        time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
			wantToday: "2026-12-31",
			wantNow:   "2026-12-31T23:59:59Z",
			wantPlus:  "2027-01-01",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sb strings.Builder
			rt := NewReader(ReadDeps{World: store.TrivialScope()}, &sb,
				WithClock(func() time.Time { return tt.at }))
			defer rt.Close()

			require.NoError(t, rt.RunString(`rela.output({
				today = rela.today,
				now = rela.now,
				now_unix = rela.now_unix,
				plus = rela.date_add(rela.now, "1d"),
			})`))

			var got struct {
				Today   string `json:"today"`
				Now     string `json:"now"`
				NowUnix int64  `json:"now_unix"`
				Plus    string `json:"plus"`
			}
			require.NoError(t, json.Unmarshal([]byte(sb.String()), &got))
			assert.Equal(t, tt.wantToday, got.Today)
			assert.Equal(t, tt.wantNow, got.Now)
			assert.Equal(t, tt.at.Unix(), got.NowUnix)
			assert.Equal(t, tt.wantPlus, got.Plus)
		})
	}
}

// TestNowDefaultsToWallClock pins the production path: without WithClock,
// rela.now is the current UTC time.
func TestNowDefaultsToWallClock(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	before := time.Now().Truncate(time.Second)
	rt := NewReader(ReadDeps{World: store.TrivialScope()}, &sb)
	defer rt.Close()
	after := time.Now()

	require.NoError(t, rt.RunString(`rela.output(rela.now)`))
	var raw string
	require.NoError(t, json.Unmarshal([]byte(sb.String()), &raw))
	got, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(raw, "Z"), "rela.now must be UTC: %s", raw)
	assert.False(t, got.Before(before) || got.After(after), "rela.now %s outside [%s, %s]", got, before, after)
}

// TestDateHelpersUseRuntimeClock pins that rela.days_since and the default
// "after" of rela.rrule_next read the same clock as rela.today, so a test that
// pins WithClock gets stable results from them too.
func TestDateHelpersUseRuntimeClock(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		script string
		want   any
	}{
		{"days_since past", `rela.output(rela.days_since("2026-11-01T12:00:00Z"))`, float64(5)},
		{"days_since future", `rela.output(rela.days_since("2026-11-08T12:00:00Z"))`, float64(-2)},
		{"rrule_next default after", `rela.output(rela.rrule_next("FREQ=WEEKLY;BYDAY=MO"))`, "2026-11-09"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sb strings.Builder
			rt := NewReader(ReadDeps{World: store.TrivialScope()}, &sb,
				WithClock(func() time.Time { return at }))
			defer rt.Close()

			require.NoError(t, rt.RunString(tt.script))
			var got any
			require.NoError(t, json.Unmarshal([]byte(sb.String()), &got))
			assert.Equal(t, tt.want, got)
		})
	}
}
