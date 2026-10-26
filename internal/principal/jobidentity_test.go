package principal_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestValidJobIdentity pins the job-identity rule the CLI's --run-as and a
// realtime session's run_as share.
func TestValidJobIdentity(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{"system:breach-bot", true},
		{"system:ci:nightly", true},
		{"integration:breach-bot", true},
		{"alice", false},
		{principal.Unknown, false},
		{"", false},
		{"system:Breach Bot", false},
		{"system:a::b", false},
		{"system:", false},
		{"integration:", false},
		{" system:ci", false},
		{principal.UserRealtime, false},
		{principal.UserScheduler, false},
		{principal.UserAutomation, false},
		{"system:webhook:x", false},
		{"system:subscription:x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := principal.ValidJobIdentity(tc.name)
			if (err == nil) != tc.ok {
				t.Fatalf("ValidJobIdentity(%q) = %v; want ok=%v", tc.name, err, tc.ok)
			}
		})
	}
}
