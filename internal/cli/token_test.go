package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
)

func TestTokenStatusRow(t *testing.T) {
	exp := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		st    tokenstore.Status
		state string
	}{
		{"missing", tokenstore.Status{Name: "crm", Declared: true}, "missing"},
		{"fresh", tokenstore.Status{Name: "crm", Stored: true, AccessFresh: true, ExpiresAt: exp}, "ok"},
		{"stale access", tokenstore.Status{Name: "crm", Stored: true}, "ok"},
		{"consent", tokenstore.Status{Name: "crm", Stored: true, NeedsConsentAt: exp}, "needs_consent"},
		{"corrupt", tokenstore.Status{Name: "crm", Err: tokenstore.ErrCorrupt}, "unreadable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.state, tokenStatusRow(tc.st).State)
		})
	}
	require.Contains(t, tokenReadReason(errors.New("io")), "io")
}

// A project with no token store answers every token command with the
// reason, not a nil dereference.
func TestTokenCmd_NotConfigured(t *testing.T) {
	w := &writeServices{TokensErr: errors.New("tokens need the sqlite or postgres backend")}
	for _, cmd := range []interface {
		Run(context.Context, *writeServices) error
	}{&TokenSetCmd{Name: "crm"}, &TokenStatusCmd{}, &TokenDeleteCmd{Name: "crm"}} {
		require.ErrorContains(t, cmd.Run(context.Background(), w), "sqlite or postgres")
	}
	require.ErrorIs(t, (&TokenStatusCmd{}).Run(context.Background(), &writeServices{}), tokenstore.ErrNotConfigured)
}
