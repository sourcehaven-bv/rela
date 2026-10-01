package visibility

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The script readers and the MCP tools name an entity by address alone. The
// [Resolver] gates on a claimed type, and the claim must be the STORED type
// (BUG-ZWTDH9), so these helpers read that type first with one raw,
// content-free header read over every face, then run the typed gates.
//
// Only the uniform miss leaves them. A missing id, a failed type read, a
// resolver miss and a GATE FAILURE all answer the same. The typed resolver
// may return a gate error because it gates before any read; here the gate
// runs only for an id the header read found, under that id's type, so a gate
// error would tell an existing id from a missing one, and its type too. It
// is logged instead.

// storedType returns the type id is stored under. A failed read is logged and
// answered as a miss, like every other resolver load (RR-FE1EGP).
func (r *Resolver) storedType(ctx context.Context, id string) (string, bool) {
	headers, ok := r.headersOf(ctx, "", id)
	if !ok || len(headers) == 0 {
		return "", false
	}
	return headers[0].Type, true
}

// addressAny is [Resolver.Address] for a caller that does not know the type.
// Every miss is [store.ErrNotFound], which the script bindings already
// translate into nil. An address the grammar refuses is a miss (ruling 9.3).
func (r *Resolver) addressAny(ctx context.Context, w World, addr string) (*entity.Entity, error) {
	ref, err := entity.ParseRef(addr)
	if err != nil {
		return nil, store.ErrNotFound
	}
	typ, ok := r.storedType(ctx, ref.ID)
	if !ok {
		return nil, store.ErrNotFound
	}
	res, ok, err := r.Address(ctx, w, typ, addr)
	if err != nil {
		warnGate("address", typ, ref.String(), err)
		return nil, store.ErrNotFound
	}
	if !ok {
		return nil, store.ErrNotFound
	}
	return res.Entity, nil
}

// writeTargetAny is [Resolver.WriteTarget] for a caller that does not know
// the type. Every miss is [store.ErrNotFound]; a bare id that picks no single
// face is the resolver's [*AmbiguousAddressError], which lists only faces the
// caller may read.
func (r *Resolver) writeTargetAny(ctx context.Context, w World, addr string) (entity.Ref, error) {
	parsed, err := entity.ParseAddress(addr)
	if err != nil {
		return entity.Ref{}, store.ErrNotFound
	}
	typ, ok := r.storedType(ctx, parsed.ID())
	if !ok {
		return entity.Ref{}, store.ErrNotFound
	}
	ref, ok, err := r.WriteTarget(ctx, w, typ, parsed)
	if _, ambiguous := errors.AsType[*AmbiguousAddressError](err); ambiguous {
		return entity.Ref{}, err
	}
	if err != nil {
		warnGate("write target", typ, addr, err)
		return entity.Ref{}, store.ErrNotFound
	}
	if !ok {
		return entity.Ref{}, store.ErrNotFound
	}
	return ref, nil
}

// familyAny is [Resolver.Family] for a caller that does not know the type,
// with one header read. id must be a bare id: an address naming a face is not
// an entity id, so it is a miss. It never returns an error; the error result
// keeps the signature of [Resolver.Family].
func (r *Resolver) familyAny(ctx context.Context, id string) (Family, bool, error) {
	if ref, err := entity.ParseRef(id); err != nil || !ref.Face.IsImplicit() {
		return Family{}, false, nil //nolint:nilerr // a refused address is a miss, not a failure (9.3)
	}
	headers, ok := r.headersOf(ctx, "", id)
	if !ok || len(headers) == 0 {
		return Family{}, false, nil
	}
	typ := headers[0].Type
	faces, ok, err := r.admit(ctx, World{}, typ, id)
	if err != nil {
		warnGate("family", typ, id, err)
		return Family{}, false, nil
	}
	if !ok {
		return Family{}, false, nil
	}
	return r.familyOf(typ, id, faces, headers)
}

// warnGate logs a gate failure the untyped helpers answer as a miss.
func warnGate(mode, entityType, addr string, err error) {
	slog.Warn("visibility: gate failed on an untyped read; answering not-found",
		"mode", mode, "type", entityType, "addr", addr, "err", err)
}
