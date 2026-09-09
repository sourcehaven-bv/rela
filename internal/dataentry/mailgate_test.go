package dataentry

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// fakeScripts serves script bodies from a map, standing in for the actions/
// directory. A missing entry reads as an unreadable script.
func fakeScripts(bodies map[string]string) scriptReader {
	return func(_, scriptPath string) (string, error) {
		body, ok := bodies[scriptPath]
		if !ok {
			return "", errors.New("script not found")
		}
		return body, nil
	}
}

func TestCollectUngatedMailActions(t *testing.T) {
	t.Parallel()

	granted := metamodel.Capabilities{Mail: true}

	tests := []struct {
		name    string
		actions map[string]dataentryconfig.Action
		bodies  map[string]string
		want    []string
	}{
		{
			name:    "mails without the grant is reported",
			actions: map[string]dataentryconfig.Action{"digest": {Script: "digest.lua"}},
			bodies:  map[string]string{"digest.lua": `mail.send{to = "a@example.com"}`},
			want:    []string{"digest (digest.lua)"},
		},
		{
			name: "mails WITH the grant is silent",
			actions: map[string]dataentryconfig.Action{
				"digest": {Script: "digest.lua", Capabilities: granted},
			},
			bodies: map[string]string{"digest.lua": `mail.send{to = "a@example.com"}`},
			want:   nil,
		},
		{
			name:    "a script that does not mail is silent",
			actions: map[string]dataentryconfig.Action{"tidy": {Script: "tidy.lua"}},
			bodies:  map[string]string{"tidy.lua": `rela.update_entity("x", {})`},
			want:    nil,
		},
		{
			// A set-only action has no body to scan and must not warn — and
			// must not panic on the empty path either.
			name:    "a set-only action is skipped",
			actions: map[string]dataentryconfig.Action{"close": {Set: map[string]string{"status": "done"}}},
			bodies:  nil,
			want:    nil,
		},
		{
			// The lint runs after the existence check, so this state means
			// something odd; either way a lint must not be what breaks boot.
			name:    "an unreadable script is skipped, not reported",
			actions: map[string]dataentryconfig.Action{"gone": {Script: "gone.lua"}},
			bodies:  nil,
			want:    nil,
		},
		{
			// Sorted output: map iteration order would otherwise make an
			// unchanged config produce a different log line on every restart.
			name: "several actions are reported in a stable order",
			actions: map[string]dataentryconfig.Action{
				"zeta":  {Script: "z.lua"},
				"alpha": {Script: "a.lua"},
				"mid":   {Script: "m.lua", Capabilities: granted},
			},
			bodies: map[string]string{
				"z.lua": "mail.send{}", "a.lua": "mail.send{}", "m.lua": "mail.send{}",
			},
			want: []string{"alpha (a.lua)", "zeta (z.lua)"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := collectUngatedMailActions(tc.actions, "/project", fakeScripts(tc.bodies))
			require.Equal(t, tc.want, got)
		})
	}
}

// TestCollectUngatedMailActions_OverReportsRatherThanUnder pins the direction
// of the heuristic's inaccuracy, because that direction is the design.
//
// A commented-out call warns (harmless: one glance at a config file), while an
// aliased call does not (also harmless: the runtime gate is exact and denies it
// regardless). Pinning both means a later "improvement" that makes the scan
// stricter has to acknowledge it is trading a cheap false positive for a false
// sense of completeness.
func TestCollectUngatedMailActions_OverReportsRatherThanUnder(t *testing.T) {
	t.Parallel()

	t.Run("a mention in a comment is reported", func(t *testing.T) {
		t.Parallel()
		got := collectUngatedMailActions(
			map[string]dataentryconfig.Action{"a": {Script: "a.lua"}}, "/project",
			fakeScripts(map[string]string{"a.lua": "-- we used to mail.send here\n"}))
		require.Equal(t, []string{"a (a.lua)"}, got)
	})

	t.Run("an aliased call is missed, and the runtime gate is the backstop", func(t *testing.T) {
		t.Parallel()
		got := collectUngatedMailActions(
			map[string]dataentryconfig.Action{"a": {Script: "a.lua"}}, "/project",
			fakeScripts(map[string]string{"a.lua": "local m = mail\nm.send{}\n"}))
		require.Empty(t, got, "the scan is a hint; missing this costs nothing because the send is denied anyway")
	})
}
