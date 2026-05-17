package audit_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

func TestPrincipalFrom_DefaultsToUnknown(t *testing.T) {
	got := audit.PrincipalFrom(context.Background())
	want := audit.Principal{User: "unknown", Tool: "unknown"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestWithPrincipal_RoundTrip(t *testing.T) {
	p := audit.Principal{User: "alice", Tool: audit.ToolCLI}
	ctx := audit.WithPrincipal(context.Background(), p)
	if got := audit.PrincipalFrom(ctx); got != p {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, p)
	}
}

func TestTriggeredByFrom_DefaultsToEmpty(t *testing.T) {
	if got := audit.TriggeredByFrom(context.Background()); got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestWithTriggeredBy_RoundTrip(t *testing.T) {
	ctx := audit.WithTriggeredBy(context.Background(), "automation:foo")
	if got := audit.TriggeredByFrom(ctx); got != "automation:foo" {
		t.Errorf("got %q, want %q", got, "automation:foo")
	}
}

func TestPrincipal_DoesNotOverrideOnRederive(t *testing.T) {
	// Demonstrates: when a cascade wraps ctx with WithTriggeredBy, the
	// originator's Principal is preserved (the cascade does NOT overwrite
	// it). This is the behavior step 5 of the technical approach relies on.
	original := audit.Principal{User: "alice", Tool: audit.ToolCLI}
	ctx := audit.WithPrincipal(context.Background(), original)
	cascade := audit.WithTriggeredBy(ctx, "automation:on-create")

	if got := audit.PrincipalFrom(cascade); got != original {
		t.Errorf("Principal was overwritten by triggered-by wrap: got %+v, want %+v", got, original)
	}
	if got := audit.TriggeredByFrom(cascade); got != "automation:on-create" {
		t.Errorf("triggered-by missing: got %q", got)
	}
}
