package filter_test

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/filter"
)

func TestResolveDateVariable(t *testing.T) {
	now := time.Date(2026, 11, 16, 23, 30, 15, 500, time.FixedZone("X", -2*3600))
	// In UTC that is 2026-11-17T01:30:15Z.
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"$today", "2026-11-17", true},
		{"$now", "2026-11-17T01:30:15Z", true},
		{"$tomorrow", "2026-11-18", true},
		{"$yesterday", "2026-11-16", true},
		{"$today+7d", "2026-11-24", true},
		{"$today-2w", "2026-11-03", true},
		{"$today+0d", "2026-11-17", true},
		{"$now+1d", "$now+1d", false},
		{"$now-1w", "$now-1w", false},
		{"$today+1m", "$today+1m", false},
		{"$today+7", "$today+7", false},
		{"$today+1234567d", "$today+1234567d", false},
		{"$today+150000w", "$today+150000w", false},
		{"$today+1000001d", "$today+1000001d", false},
		{"$todayx", "$todayx", false},
		{"$today*", "$today*", false},
		{"$TODAY", "$TODAY", false},
		{" $today", " $today", false},
		{"today", "today", false},
		{"$tomorrow+1d", "$tomorrow+1d", false},
		{"$user", "$user", false},
		{"", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := filter.ResolveDateVariable(tc.in, now)
			if got != tc.want || ok != tc.ok {
				t.Errorf("ResolveDateVariable(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestBindDateVariables(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	fs, err := filter.ParseAll([]string{"due<$today", "status=open"})
	if err != nil {
		t.Fatal(err)
	}
	fs = append(fs, nil)
	got := filter.BindDateVariables(fs, now)
	if got[0].Value != "2026-11-16" {
		t.Errorf("bound value = %q", got[0].Value)
	}
	if fs[0].Value != "$today" {
		t.Errorf("input was modified: %q", fs[0].Value)
	}
	if got[1] != fs[1] {
		t.Error("a filter without a variable should be kept as is")
	}
	if got[2] != nil {
		t.Error("nil filter not kept")
	}
}
