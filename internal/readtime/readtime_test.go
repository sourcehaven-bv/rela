package readtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/readtime"
)

func TestReadTime(t *testing.T) {
	ams := time.FixedZone("CET", 3600)
	first := time.Date(2026, 11, 16, 23, 30, 15, 999, ams)
	second := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	wall := func() time.Time { return second }

	tests := []struct {
		name string
		ctx  func() context.Context
		want time.Time
		has  bool
	}{
		{"none: Or falls back", context.Background, second, false},
		{"With normalizes to UTC seconds", func() context.Context {
			return readtime.With(context.Background(), first)
		}, time.Date(2026, 11, 16, 22, 30, 15, 0, time.UTC), true},
		{"With replaces", func() context.Context {
			return readtime.With(readtime.With(context.Background(), first), second)
		}, second, true},
		{"Ensure keeps an existing time", func() context.Context {
			return readtime.Ensure(readtime.With(context.Background(), first), wall)
		}, time.Date(2026, 11, 16, 22, 30, 15, 0, time.UTC), true},
		{"Ensure stamps when absent", func() context.Context {
			return readtime.Ensure(context.Background(), wall)
		}, second, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.ctx()
			if _, ok := readtime.From(ctx); ok != tc.has {
				t.Errorf("From ok = %v, want %v", ok, tc.has)
			}
			if got := readtime.Or(ctx, wall); !got.Equal(tc.want) || got.Location() != time.UTC {
				t.Errorf("Or = %v, want %v in UTC", got, tc.want)
			}
		})
	}
}

func TestFromNilContext(t *testing.T) {
	//nolint:staticcheck // SA1012: From documents nil as accepted.
	if _, ok := readtime.From(nil); ok {
		t.Error("From(nil) reported a read time")
	}
}
