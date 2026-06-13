package dataentry

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/search"
)

// readGate is the dataentry-package consumer-side interface for ACL
// read decisions. Centralizing the surface here means every read
// chokepoint (per-entity GET, ?include= filter, write-path 404 parity,
// and future list / sidebar / _position consumers) calls the same
// boolean rather than threading `acl.FromContext(ctx)` plus a
// nil-check plus a per-call switch through each handler.
//
// Three flavors of decision, all phrased as ACL permission questions:
//
//   - PermitsRead(ctx, type, id) — single-entity probe. Used by GET,
//     write paths, and per-id include checks.
//   - PermitsReadMany(ctx, type, ids) — batched probe returning a
//     permission map. Used by the ?include= filter and any future
//     list consumer.
//   - ReadQuery(ctx, type) — list-scope verdict. Used by the list
//     pipeline (scopedSortedEntities) and the sidebar counts to decide
//     between unfiltered (AllowAll), empty (DenyAll), and a composed
//     store.GraphQuery that selects the visible subset (TKT-VMD8).
//   - SearchScope(ctx, types) — the mixed-type search scope: ReadQuery
//     resolved over every metamodel type, shaped for
//     search.VisibleSearcher (TKT-BA8BSX). The gate owns the nop/ACL
//     distinction: the nop gate returns the wildcard-allow scope so
//     off-metamodel entities stay visible exactly as before ACL
//     existed, while the ACL gate emits per-type entries only — an
//     entity type the metamodel doesn't know fails closed.
//
// None of the methods verify existence — they answer "the policy
// permits reading this id IF it exists". Callers that need existence
// follow up with getEntity.
//
// The production impl (aclReadGate) wraps a per-request *acl.Request;
// the no-op impl (nopReadGate) is what handlers get when no ACL is
// configured.
type readGate interface {
	PermitsRead(ctx context.Context, entityType, entityID string) (bool, error)
	PermitsReadMany(ctx context.Context, entityType string, ids []string) (map[string]bool, error)
	ReadQuery(ctx context.Context, entityType string) acl.ReadQueryResult
	SearchScope(ctx context.Context, types []string) map[string]search.TypeScope
}

// aclReadGate is the production implementation of readGate. Wraps a
// per-request *acl.Request constructed by attachACLRequest so the
// member-of cache is shared across every gate call in one HTTP
// request.
type aclReadGate struct {
	req *acl.Request
}

// newACLReadGate constructs an aclReadGate, rejecting a nil Request.
// The two production wiring sites (attachACLRequest, both branches)
// and any future caller MUST go through this constructor so a nil
// Request can never silently produce a gate that panics on use.
func newACLReadGate(r *acl.Request) (readGate, error) {
	if r == nil {
		return nil, errors.New("dataentry: newACLReadGate: acl.Request is nil")
	}
	return aclReadGate{req: r}, nil
}

func (g aclReadGate) PermitsRead(ctx context.Context, entityType, entityID string) (bool, error) {
	return g.req.PermitsRead(ctx, entityType, entityID)
}

func (g aclReadGate) PermitsReadMany(ctx context.Context, entityType string, ids []string) (map[string]bool, error) {
	return g.req.PermitsReadMany(ctx, entityType, ids)
}

func (g aclReadGate) ReadQuery(ctx context.Context, entityType string) acl.ReadQueryResult {
	return g.req.ReadQuery(ctx, entityType)
}

// SearchScope maps per-type ReadQuery verdicts onto the
// search.TypeScope shape: AllowAll and Query verdicts become entries,
// DenyAll types are simply absent (absence IS the deny in the seam's
// fail-closed lookup), and no wildcard is ever emitted — an entity
// type outside the metamodel cannot be granted by a policy, so it
// must not be visible through search either.
func (g aclReadGate) SearchScope(ctx context.Context, types []string) map[string]search.TypeScope {
	scope := make(map[string]search.TypeScope, len(types))
	for _, typ := range types {
		rqr := g.req.ReadQuery(ctx, typ)
		switch {
		case rqr.AllowAll:
			scope[typ] = search.TypeScope{AllowAll: true}
		case rqr.Query != nil:
			scope[typ] = search.TypeScope{Query: rqr.Query}
		}
	}
	return scope
}

// nopReadGate answers "permitted" for every probe. It's the gate the
// handlers see under NopACL / ReadOnlyACL — the wire response shape
// is then byte-identical to today's pre-ACL behavior, which is what
// the NopACL regression test pins (TKT-VQGN AC6).
type nopReadGate struct{}

func (nopReadGate) PermitsRead(context.Context, string, string) (bool, error) {
	return true, nil
}

func (nopReadGate) PermitsReadMany(_ context.Context, _ string, ids []string) (map[string]bool, error) {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m, nil
}

func (nopReadGate) ReadQuery(context.Context, string) acl.ReadQueryResult {
	return acl.ReadQueryResult{AllowAll: true}
}

// SearchScope under no ACL is the wildcard-allow scope — NOT one
// AllowAll entry per metamodel type. The difference is entities whose
// type is absent from the metamodel (permissive storage tolerates
// them): they were searchable before ACL existed and must stay so
// when no policy is configured (NopACL byte-parity, TKT-BA8BSX AC9).
func (nopReadGate) SearchScope(context.Context, []string) map[string]search.TypeScope {
	return map[string]search.TypeScope{search.WildcardType: {AllowAll: true}}
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
// become permits-all on a misconfigured chain (the fail-open shape
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
