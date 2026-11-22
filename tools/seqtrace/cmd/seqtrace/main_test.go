package main

import (
	"regexp"
	"testing"
)

func TestDefaultTitle(t *testing.T) {
	re := regexp.MustCompile(defaultTitle)
	for _, tc := range []struct {
		fn   string
		want bool
	}{
		{"(*copiesHandler).handleV1Copies", true},
		{"handleV1Audit", true},
		{"requestStats.func1", false},
		{"permitsAuditRead", false},
		{"rehandle", false},
		{"handle.Transact", false},
	} {
		t.Run(tc.fn, func(t *testing.T) {
			if got := re.MatchString(tc.fn); got != tc.want {
				t.Errorf("match(%q) = %v, want %v", tc.fn, got, tc.want)
			}
		})
	}
}
