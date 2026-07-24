package entitymanager

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestWithStoreAttribution pins the boundary translation (RR-U964M0): only a
// real principal becomes a store.Attribution; the zero principal and the
// {unknown, unknown} fallback principal.From returns for an unstamped ctx must
// leave the ctx without attribution, so backends persist NULL and the version
// sweep keeps its system-principal fallback.
func TestWithStoreAttribution(t *testing.T) {
	tests := []struct {
		name      string
		principal *principal.Principal // nil = unstamped ctx
		want      store.Attribution
	}{
		{
			name:      "unstamped ctx forwards nothing",
			principal: nil,
			want:      store.Attribution{},
		},
		{
			name:      "explicit unknown-unknown principal forwards nothing",
			principal: &principal.Principal{User: "unknown", Tool: "unknown"},
			want:      store.Attribution{},
		},
		{
			name:      "real principal forwards user and tool",
			principal: &principal.Principal{User: "alice@example.com", Tool: "data-entry"},
			want:      store.Attribution{User: "alice@example.com", Tool: "data-entry"},
		},
		{
			name:      "unknown user with real tool still forwards",
			principal: &principal.Principal{User: "unknown", Tool: "cli"},
			want:      store.Attribution{User: "unknown", Tool: "cli"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.principal != nil {
				ctx = principal.With(ctx, *tc.principal)
			}
			got := store.AttributionFrom(withStoreAttribution(ctx))
			if got != tc.want {
				t.Fatalf("attribution = %+v, want %+v", got, tc.want)
			}
		})
	}
}
