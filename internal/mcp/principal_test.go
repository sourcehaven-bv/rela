package mcp

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

// TestPrincipalContext_WithoutOption verifies that a Server constructed
// without WithPrincipal does not wrap the ctx (returns it unchanged).
// This keeps tests that don't care about audit attribution simple.
func TestPrincipalContext_WithoutOption(t *testing.T) {
	s := &Server{}
	ctx := context.Background()
	got := s.principalContext(ctx)
	if got != ctx {
		t.Errorf("expected unchanged ctx, got a wrapped one")
	}
	// And no Principal is found on it.
	p := audit.PrincipalFrom(got)
	if p.User != "unknown" || p.Tool != "unknown" {
		t.Errorf("unexpected Principal: %+v", p)
	}
}

// TestPrincipalContext_WithOption verifies that the WithPrincipal
// option stamps the supplied Principal on every ctx passed through
// principalContext. AC4 for MCP entry point.
func TestPrincipalContext_WithOption(t *testing.T) {
	want := audit.Principal{User: "alice", Tool: audit.ToolMCP}
	s := &Server{}
	WithPrincipal(want)(s)

	got := audit.PrincipalFrom(s.principalContext(context.Background()))
	if got != want {
		t.Errorf("Principal = %+v, want %+v", got, want)
	}
}
