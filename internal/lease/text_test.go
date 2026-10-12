package lease

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestUserText(t *testing.T) {
	t.Parallel()
	invalid := fmt.Errorf("%w: label too long", ErrInvalid)
	forbidden := &acl.ForbiddenError{}
	ambiguous := &AmbiguousError{Err: errors.New("DOC-1 has faces; address one: DOC-1@a, DOC-1@b")}
	held := &HeldError{Current: Lease{
		ID:      "DOC-1",
		Holder:  Holder{Identity: "secret", User: "bob", Tool: "mcp", Label: "agent-1"},
		Since:   t0,
		Expires: t0.Add(DefaultTTL),
		Status:  "coding",
	}}
	cases := []struct {
		name string
		err  error
		want string
		ok   bool
	}{
		{"not enabled", ErrNotEnabled, NotEnabledText, true},
		{"lost", ErrLost, LostText, true},
		{"wrapped lost", fmt.Errorf("x: %w", ErrLost), LostText, true},
		{"full", ErrFull, FullText, true},
		{"limit", ErrLimit, limitText("HINT"), true},
		{"invalid", invalid, invalid.Error(), true},
		{"no principal", ErrNoPrincipal, ErrNoPrincipal.Error(), true},
		{"forbidden", forbidden, forbidden.Error(), true},
		{"ambiguous", fmt.Errorf("claim: %w", ambiguous), "claim: " + ambiguous.Error(), true},
		{"held", held, HeldPrefix + `{"holder":{"user":"bob","tool":"mcp","label":"agent-1"},` +
			`"since":"2026-10-01T12:00:00Z","expires_at":"2026-10-01T12:15:00Z","status":"coding"}`, true},
		{"not found is the caller's", store.ErrNotFound, "", false},
		{"unexpected", errors.New("disk on fire"), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := UserText(tc.err, "HINT")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
	t.Run("limit text cites the hint and the shared cap", func(t *testing.T) {
		t.Parallel()
		got, _ := UserText(ErrLimit, "`rela lease list`")
		require.Contains(t, got, "(`rela lease list` shows them)")
		require.Contains(t, got, "32 live leases")
		require.Contains(t, got, "every agent running as you")
	})
	t.Run("held text has no identity", func(t *testing.T) {
		t.Parallel()
		got, _ := UserText(held, "")
		require.NotContains(t, got, "secret")
	})
}
