package mcp

import (
	"context"
	"sync/atomic"
)

// snapshot is the reloadable core of a [Server]: the dependency bundle and the
// handler groups derived from it. The two MUST move together — [handlerSet] is
// built from a [Deps] and caches its metamodel inside typeResolver,
// schemaResourceHandler and promptHandler, so publishing them independently
// would let a request observe new deps with handlers still bound to the old
// metamodel.
//
// Published via [snapshotProvider], following the CLAUDE.md state-publish rule:
// an atomic.Pointer for the snapshot, so readers never take a lock and never
// observe a torn pair.
type snapshot struct {
	deps     Deps
	handlers handlerSet
}

// newSnapshot derives the handler groups for d and pairs them with it.
func newSnapshot(d Deps) *snapshot {
	return &snapshot{deps: d, handlers: d.handlers()}
}

// snapshotProvider publishes the current [snapshot]. Readers Load lock-free;
// the reload path Stores a freshly-derived snapshot atomically.
//
// Owning the pointer in its own type (rather than as a bare field on Server)
// keeps the publish mechanics in one place, and means a Server built as a
// literal in a test can publish a snapshot with one call rather than
// remembering to set two fields consistently.
type snapshotProvider struct {
	ptr atomic.Pointer[snapshot]
}

// current returns the published snapshot. Nil only before the first publish,
// which [NewServer] performs — a running Server always has one.
func (p *snapshotProvider) current() *snapshot { return p.ptr.Load() }

// publish installs s as the current snapshot.
func (p *snapshotProvider) publish(s *snapshot) { p.ptr.Store(s) }

// bind defers handler-group resolution to REQUEST time.
//
// Registration used to pass a method value (`s.trace.handleTraceFrom`) straight
// to AddTool. A method value captures its receiver by value at the moment it is
// formed, so every registered handler held the handler group — and through it
// the metamodel — that existed at startup. A reloaded snapshot could never
// reach them, and the failure would be silent: the tool keeps working, just
// against the old schema.
//
// bind takes a selector for the group and the method to call on it, and
// returns a handler that resolves the group per request. `bind(s, selTrace,
// traceHandler.handleTraceFrom)` reads much like the old expression but
// re-reads the snapshot on every call.
//
// Generic over the group and over the request/result types so one helper covers
// tools, resources and prompts alike.
func bind[G, Req, Res any](
	s *Server, sel func(handlerSet) G, method func(G, context.Context, Req) (Res, error),
) func(context.Context, Req) (Res, error) {
	return func(ctx context.Context, req Req) (Res, error) {
		return method(group(s, sel), ctx, req)
	}
}
