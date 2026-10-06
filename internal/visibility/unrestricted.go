package visibility

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// UnrestrictedReader is a script read handle that applies NO gate and NO
// redaction: every read goes straight to the store. Build one with
// [Unrestricted].
//
// Why this type exists at all, given it does nothing: an ungated wiring
// must be spelled out. Three production sites were once silently ungated
// because a bare store satisfied the Lua read surface, so `VisibleReader: st`
// looked as deliberate as a gated reader (RR-R0G3DF). The Lua read surface
// (internal/lua's EntityReader) now reads through GetAddress, which
// store.Store does not have, so the compiler refuses a bare store and an
// ungated path must name this type:
//
//	grep -rn "visibility.Unrestricted" --include=*.go
//
// enumerates every ungated script read path in the tree, in one command.
//
// Deliberately NOT a store.Store: it exposes read methods only, so it
// cannot be passed where a full store is wanted (a write path, say) and
// cannot silently widen back into one.
type UnrestrictedReader struct {
	st    store.Store
	res   *Resolver
	world World
}

// Unrestricted wraps a raw store as an explicitly ungated script read
// handle.
//
// It PANICS on a nil store, per CLAUDE.md "constructors reject nil
// required fields". Returning a nil *UnrestrictedReader instead would be
// actively dangerous: assigning a typed nil pointer into the
// lua.EntityReader interface field produces a NON-nil interface, so lua's
// `VisibleReader == nil` deny guard (runtime.go, RR-X9NVHI) is skipped and
// the first read nil-derefs inside GetAddress. Nothing on the script paths
// recovers, so that panic takes the process down at request time rather
// than raising the clean "no reader is configured" Lua error the deny path
// produces. A nil store here is a wiring bug: failing loudly at
// construction, in the stack of the code that made the mistake, is
// strictly better than either alternative.
//
// Legitimate uses are paths where an ACL cannot apply or would be wrong:
//
//   - The operator trust boundary — CLI and docs runtimes, where whoever
//     runs the binary already has the project files, so gating the reads
//     would protect nothing (RR-17DMC).
//   - Validation rule bodies, where redacting incidental cross-entity
//     lookups manufactures false violations rather than hiding anything:
//     a rule asserting "every ticket links to a project" would fire on
//     projects the acting principal cannot see.
//   - A throwaway store the caller just seeded itself.
//
// If a new call site does not clearly fall into one of those, it probably
// wants the ACL-bound reader instead — see [NewScriptReader].
//
// opts configure its [Resolver]; a wiring site with a metamodel passes
// [WithFamilies]. A bad option panics for the same reason a nil store does.
func Unrestricted(st store.Store, opts ...ResolverOption) *UnrestrictedReader {
	if st == nil {
		panic("visibility.Unrestricted: store must be non-nil")
	}
	res, err := NewAllowAllResolver(st, opts...)
	if err != nil {
		panic("visibility.Unrestricted: " + err.Error())
	}
	return &UnrestrictedReader{st: st, res: res}
}

// WithWorld returns a copy of r whose bare-id reads resolve in w. Until the
// wiring sets it, the world is unset and a bare-id read fails closed.
func (r *UnrestrictedReader) WithWorld(w World) *UnrestrictedReader {
	c := *r
	c.world = w
	return &c
}

// GetAddress implements the script read surface. It resolves addr exactly as
// the gated [ScriptReader.GetAddress] does, through the allow-all [Resolver]:
// `ID@face` reads that face, a bare id resolves in the reader's world, and an
// address the grammar refuses misses. Only the gate and the redaction are
// absent.
func (r *UnrestrictedReader) GetAddress(ctx context.Context, addr string) (*entity.Entity, error) {
	return r.res.addressAny(ctx, worldIn(ctx, r.world), addr)
}

// WriteTarget resolves addr to the face a write edits, through the allow-all
// resolver. Like [ScriptReader.WriteTarget] it ignores a per-operation read
// world.
func (r *UnrestrictedReader) WriteTarget(ctx context.Context, addr string) (entity.Ref, error) {
	return r.res.writeTargetAny(ctx, r.world, addr)
}

// Family reports every stored face of the entity id, reading headers only.
// See [ScriptReader.Family].
func (r *UnrestrictedReader) Family(ctx context.Context, id string) (Family, bool, error) {
	return r.res.familyAny(ctx, id)
}

// ResolveHeaders answers a batch of addresses from headers only. See
// [ScriptReader.ResolveHeaders].
func (r *UnrestrictedReader) ResolveHeaders(ctx context.Context, refs []entity.Ref) map[entity.Ref]ResolvedHeader {
	return r.res.ResolveHeaders(ctx, worldIn(ctx, r.world), refs)
}

// ListEntities implements the script read surface: straight pass-through.
func (r *UnrestrictedReader) ListEntities(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	return r.st.ListEntities(ctx, q)
}

// ListEntityHeaders implements the script read surface: straight
// pass-through to the store's header projection (or its fallback).
func (r *UnrestrictedReader) ListEntityHeaders(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[store.EntityHeader, error] {
	return store.ListEntityHeaders(ctx, r.st, q)
}

// ListRelations implements the script read surface: straight pass-through.
func (r *UnrestrictedReader) ListRelations(
	ctx context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return r.st.ListRelations(ctx, q)
}

// ListRelationsStrict is ListRelations: there is no gate to fault.
func (r *UnrestrictedReader) ListRelationsStrict(
	ctx context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return r.st.ListRelations(ctx, q)
}
