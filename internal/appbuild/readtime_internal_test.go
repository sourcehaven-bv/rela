package appbuild

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/readtime"
)

// TestBinders_ReadTime pins how the two identity binders treat the read
// time (TKT-08ZI5XN): IdentityBinder keeps a time already on ctx, so a
// nested bind inside one operation never moves it; Rebind, which a
// long-lived reader (the MCP watch) calls per round, replaces it, so the
// clock advances between rounds.
func TestBinders_ReadTime(t *testing.T) {
	earlier := time.Date(2026, 11, 15, 8, 0, 0, 0, time.UTC)
	later := time.Date(2026, 11, 16, 8, 0, 0, 0, time.UTC)
	clock := func() time.Time { return later }
	stamped := func() context.Context { return readtime.With(context.Background(), earlier) }

	tests := []struct {
		name string
		bind func(context.Context) (context.Context, error)
		ctx  func() context.Context
		want time.Time
	}{
		{"identity binder keeps", identityBinderFor(nil, clock), stamped, earlier},
		{"identity binder stamps", identityBinderFor(nil, clock), context.Background, later},
		{"rebind replaces", rebindFor(nil, clock), stamped, later},
		{"rebind stamps", rebindFor(nil, clock), context.Background, later},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, err := tc.bind(tc.ctx())
			if err != nil {
				t.Fatal(err)
			}
			got, ok := readtime.From(ctx)
			if !ok || !got.Equal(tc.want) {
				t.Errorf("read time = %v (%v), want %v", got, ok, tc.want)
			}
		})
	}
}
