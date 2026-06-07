package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// readGate is the dataentry-package consumer-side interface for ACL
// read decisions. Centralizing the surface here means every read
// chokepoint (per-entity GET, ?include= filter, write-path 404 parity,
// and future list / sidebar / _position consumers) calls the same
// boolean rather than threading `acl.FromContext(ctx)` plus a
// nil-check plus a per-call switch through each handler.
//
// Two flavors of decision:
//
//   - Visible(ctx, type, id) — single-entity probe. Used by GET, write
//     paths, and the include resolver after batching neighbor IDs by
//     type. Implemented in terms of [acl.Request.Visible].
//   - Query(ctx, type) — list-shape decision returning AllowAll /
//     DenyAll / a composed *store.GraphQuery. Used by future list
//     consumers (TKT-VMD8). Implemented in terms of
//     [acl.Request.ReadQuery].
//
// The production impl wraps *acl.Request; the no-op impl (nopReadGate)
// is what handlers get when no ACL is configured. Tests that need
// principal-specific behavior use the production impl over a tiny
// acl.yaml fixture.
type readGate interface {
	Visible(ctx context.Context, entityType, entityID string) (bool, error)
	Query(ctx context.Context, entityType string) acl.ReadQueryResult
}

// aclReadGate is the production implementation of readGate. Wraps a
// per-request *acl.Request constructed by attachACLRequest so the
// member-of cache is shared across every gate call in one HTTP
// request.
type aclReadGate struct {
	req *acl.Request
}

func (g aclReadGate) Visible(ctx context.Context, entityType, entityID string) (bool, error) {
	return g.req.Visible(ctx, entityType, entityID)
}

func (g aclReadGate) Query(ctx context.Context, entityType string) acl.ReadQueryResult {
	return g.req.ReadQuery(ctx, entityType)
}

// nopReadGate answers "visible / AllowAll" for every probe. It's the
// gate the handlers see under NopACL / ReadOnlyACL — the wire response
// shape is then byte-identical to today's pre-ACL behavior, which is
// what the NopACL regression test pins (TKT-VQGN AC6).
type nopReadGate struct{}

func (nopReadGate) Visible(context.Context, string, string) (bool, error) {
	return true, nil
}

func (nopReadGate) Query(context.Context, string) acl.ReadQueryResult {
	return acl.ReadQueryResult{AllowAll: true}
}

// readGateCtxKey is the unexported type for context.WithValue. The
// stdlib contract requires non-bare-string context keys.
type readGateCtxKey struct{}

// withReadGate attaches g to ctx so handlers can pull it via
// readGateFromContext. Wired by attachACLRequest; tests that bypass
// the middleware (calling handlers directly) attach an explicit gate
// via this helper.
func withReadGate(ctx context.Context, g readGate) context.Context {
	return context.WithValue(ctx, readGateCtxKey{}, g)
}

// readGateFromContext returns the gate attached via withReadGate, or
// nopReadGate when none is present. Handlers MUST go through this
// helper (not direct ctx.Value lookups) so the nil-handling stays in
// one place — a handler that forgets to nil-check would silently
// become an AllowAll on a misconfigured chain (the fail-open shape
// RR-875A flagged on attachACLRequest).
func readGateFromContext(ctx context.Context) readGate {
	if ctx == nil {
		return nopReadGate{}
	}
	g, ok := ctx.Value(readGateCtxKey{}).(readGate)
	if !ok || g == nil {
		return nopReadGate{}
	}
	return g
}
