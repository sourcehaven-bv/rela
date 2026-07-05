// Package principal carries the identity attribution attached to each
// request: who initiated it (User) and through what entry point
// (Tool). Entry-point binaries stamp a [Principal] on the request
// context once at startup (or per-request for HTTP-style entry
// points); downstream consumers — audit logging today, access control
// in subsequent PRs — read it via [From].
//
// This package is deliberately small. It owns the [Principal] type,
// the Tool* constants, the context plumbing, and the OS-user lookup.
// It does NOT carry session lifecycle, role mapping, or
// authentication — those are separate concerns that may grow their
// own packages.
//
// **Growth gated on ACL.** The package was extracted from `audit`
// specifically so a future ACL ticket can import [Principal] without
// pulling audit. If that ticket gets de-prioritized and no second
// consumer materializes, this package should be reabsorbed into the
// consumer that uses it most — currently `audit`. Don't grow this
// package speculatively.
package principal

import (
	"context"
	"os"
	"strings"
)

// Principal identifies who is making a write. User is the OS user
// captured at process startup via [SystemUser]; data-entry overrides it
// per-request from an HTTP middleware. Tool identifies the entry point —
// one of the Tool* constants below.
//
// RawUser records the pre-resolution identifier when the ACL policy's
// `principal_property` lookup substituted User with a graph entity ID
// (e.g. the `X-Forwarded-User` email before it became a `PERS-…` ID). It
// is empty when no substitution happened — the common case — and exists
// so the audit log can record BOTH the identity that authenticated and
// the entity it resolved to without a round-trip to the graph. Old audit
// consumers ignore the `raw_user` key (omitempty).
type Principal struct {
	User    string `json:"user"`
	Tool    string `json:"tool"`
	RawUser string `json:"raw_user,omitempty"`
}

// Tool constants — the values that may appear in [Principal.Tool].
// Entry-point wiring references these instead of string literals so
// typos surface at compile time.
const (
	ToolCLI       = "cli"
	ToolMCP       = "mcp"
	ToolDataEntry = "data-entry"
	ToolScheduler = "scheduler"
	ToolDesktop   = "desktop"
	// ToolSync attributes writes applied by the sync API (FEAT-NJ9FEN) so the
	// audit log distinguishes a synced write from a direct data-entry edit.
	ToolSync = "sync"
	// ToolWebhookReceiver attributes writes made by an inbound-webhook handler
	// (e.g. an IdP membership event that provisions a person entity). It is a
	// distinct entry point from data-entry: the write originates from a verified
	// server-to-server callback, not a human at the UI.
	ToolWebhookReceiver = "webhook-receiver"
)

// principalKey is the unexported context.WithValue key so no other
// package can collide with it or read/write the value outside this
// package's API.
type principalKey struct{}

// With returns a derived context carrying p. Entry points stamp
// Principal once at startup (or per-request for data-entry);
// consumers read it via [From] on each operation.
//
// Call sites read as `principal.With(ctx, ...)` and `principal.From(ctx)` —
// the package name carries the noun so the function names don't stutter.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// From returns the Principal carried by ctx, or
// Principal{User:"unknown", Tool:"unknown"} if none was stamped.
// Returning a default rather than panicking keeps downstream consumers
// best-effort even when a new call site forgets to thread principal
// through — the misattribution is visible (in the audit log, for
// instance), not silent.
func From(ctx context.Context) Principal {
	if v, ok := ctx.Value(principalKey{}).(Principal); ok {
		return v
	}
	return Principal{User: "unknown", Tool: "unknown"}
}

// SystemUser returns the OS user running this process — $USER
// trimmed, or "unknown" if $USER is unset or whitespace-only. Used by
// entry-point wiring to populate [Principal.User].
//
// The original plan had a four-tier fallback chain ($RELA_ACTOR →
// $USER → git config user.email → "system") — dropped during review
// as over-engineered for what is one bit of operator identity.
// Operators who want a different identity set $USER; data-entry will
// override per-request via HTTP middleware in a follow-up.
func SystemUser() string {
	u := strings.TrimSpace(os.Getenv("USER"))
	if u == "" {
		return "unknown"
	}
	return u
}
