package store_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestAttributionRoundTrip pins the ctx-carrier contract (RR-2VWA0Q): a set
// Attribution reads back verbatim, and an unset ctx yields the zero value —
// never a fabricated default.
func TestAttributionRoundTrip(t *testing.T) {
	ctx := context.Background()

	if got := store.AttributionFrom(ctx); !got.IsZero() {
		t.Fatalf("AttributionFrom on bare ctx = %+v, want zero", got)
	}

	a := store.Attribution{User: "alice@example.com", Tool: "data-entry"}
	got := store.AttributionFrom(store.WithAttribution(ctx, a))
	if got != a {
		t.Fatalf("AttributionFrom = %+v, want %+v", got, a)
	}
	if got.IsZero() {
		t.Fatal("set attribution must not report IsZero")
	}
}
