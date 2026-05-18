package audit_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

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

// TestPrincipalAndTriggeredBy_Orthogonal demonstrates that wrapping
// ctx with audit.WithTriggeredBy preserves the originator's
// principal.Principal — the two context values are independent. The
// cascade path relies on this: triggered_by gets layered on by the
// runner without overwriting the originator's Principal.
func TestPrincipalAndTriggeredBy_Orthogonal(t *testing.T) {
	original := principal.Principal{User: "alice", Tool: principal.ToolCLI}
	ctx := principal.With(context.Background(), original)
	cascade := audit.WithTriggeredBy(ctx, "automation:on-create")

	if got := principal.From(cascade); got != original {
		t.Errorf("Principal was overwritten by triggered-by wrap: got %+v, want %+v", got, original)
	}
	if got := audit.TriggeredByFrom(cascade); got != "automation:on-create" {
		t.Errorf("triggered-by missing: got %q", got)
	}
}
