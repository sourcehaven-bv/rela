package visibility

import (
	"context"
	"errors"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ScriptReader adapts a [Reader] to the read surface script runtimes
// consume (internal/lua's EntityReader, satisfied structurally so lua needs
// no dependency on this package). Every method row-gates and field-redacts
// through the wrapped Reader's policy, so a script sees exactly the caller's
// view (DEC-O59WM4).
//
// # Single-entity reads
//
// [ScriptReader.GetAddress] and [ScriptReader.Family] go through the wrapped
// Reader's [Resolver]. A script names an entity by address and no type, so
// the reader first reads the STORED type from one content-free header, then
// calls the typed resolver with it. Claiming the stored type keeps the
// BUG-ZWTDH9 cross-type surface closed. A bare id resolves in the reader's
// world ([ScriptReader.WithWorld]; unset, so failing closed, until wired), and
// `ID@face` reads that face.
//
// # Allocation
//
// ListEntities materializes the matching set to filter it (the row gate is
// batched by design — one probe per distinct type rather than one per row).
// `rela.list_entities` has no limit today, so on a large type this is
// roughly 3x the peak allocation of an ungated stream: the slice, the
// filtered slice, and the redacted copies.
//
// The field redactor still runs PER ROW (one verdict resolution each), which
// batching does not amortize (RR-3U1V80). Bounding these bindings is
// TKT-YWDGZD; this type does not paper over it.
type ScriptReader struct {
	reader Reader
	res    *Resolver
	world  World
	raw    store.Store
	binder Binder

	// provider, when non-nil, lets ListEntities push the row gate into the
	// store as a query instead of probing per batch. Derived from binder in
	// the constructor; nil simply means no pushdown, never a weaker gate.
	provider ReadQueryProvider
}

// Binder attaches a per-operation ACL scope to a ctx. [DeclarativeGate]
// implements it. Optional on [ScriptReader]: when supplied, every read
// binds once and reuses that scope for both the row gate and the field
// redactor.
type Binder interface {
	Bind(ctx context.Context) (context.Context, error)
}

// NewScriptReader wraps reader over the raw store. Both are required: raw
// supplies the underlying rows, reader decides which of them (and which of
// their properties) the caller may see. reader's [Reader.Resolver] serves
// the single-entity reads and must be non-nil.
//
// binder is optional but strongly recommended (RR-CCBZBH). Without it,
// every gate probe AND every field-verdict resolution opens its own
// acl.Request and re-walks the member-of graph — measured at ~21 walks per
// list call over 20 rows, versus 1 when bound. It also makes each read a
// single consistent ACL snapshot instead of letting the row gate and the
// redactor disagree if a write lands mid-operation. Pass the same
// [DeclarativeGate] used to build reader.
func NewScriptReader(reader Reader, raw store.Store, binder Binder) (*ScriptReader, error) {
	if reader == nil {
		return nil, errors.New("visibility: NewScriptReader: reader must be non-nil")
	}
	if raw == nil {
		return nil, errors.New("visibility: NewScriptReader: raw store must be non-nil")
	}
	res := reader.Resolver()
	if res == nil {
		return nil, errors.New("visibility: NewScriptReader: reader's Resolver must be non-nil")
	}
	s := &ScriptReader{reader: reader, res: res, raw: raw, binder: binder}
	// The binder IS the gate in every production wiring, and DeclarativeGate
	// composes the read scope as a store predicate. Deriving the provider
	// from it rather than taking a fourth constructor argument keeps the two
	// from ever disagreeing about which policy is in force — a pushdown built
	// from a different gate than the row filter would be a silent
	// authorization split.
	if p, ok := binder.(ReadQueryProvider); ok {
		s.provider = p
	}
	return s, nil
}

// WithWorld returns a copy of s whose bare-id reads resolve in w. Until the
// wiring sets it, the world is unset and a bare-id read fails closed.
func (s *ScriptReader) WithWorld(w World) *ScriptReader {
	c := *s
	c.world = w
	return &c
}

// bind opens (or reuses) the per-operation ACL scope. A bind failure —
// notably an unstamped principal — is NOT swallowed here: the caller
// proceeds with the original ctx, and the gate then denies on its own
// terms, so a failure can never widen visibility.
func (s *ScriptReader) bind(ctx context.Context) context.Context {
	if s.binder == nil {
		return ctx
	}
	bound, err := s.binder.Bind(ctx)
	if err != nil {
		return ctx
	}
	return bound
}

// GetAddress reads the face addr names (`ID@face`), or the face the reader's
// world resolves a bare id to, gated and redacted. Every miss, including a
// denied entity and an address the grammar refuses, is [store.ErrNotFound],
// so a script cannot tell hidden from absent. A gate failure is logged and
// answered the same way, because it can only occur for an id that exists.
func (s *ScriptReader) GetAddress(ctx context.Context, addr string) (*entity.Entity, error) {
	return s.res.addressAny(s.bind(ctx), worldIn(ctx, s.world), addr)
}

// WriteTarget resolves addr to the one face a face-level write edits, in
// the reader's world (see [Resolver.WriteTarget]). Every miss is
// [store.ErrNotFound]; a bare id that picks no single face is an
// [*AmbiguousAddressError] naming the faces the caller may read.
func (s *ScriptReader) WriteTarget(ctx context.Context, addr string) (entity.Ref, error) {
	return s.res.writeTargetAny(s.bind(ctx), worldIn(ctx, s.world), addr)
}

// Family reports which faces of the entity id the caller may read, reading
// headers only. It answers the entity-level question a write asks before it
// names an id: does a readable face of it exist. See [Resolver.Family].
func (s *ScriptReader) Family(ctx context.Context, id string) (Family, bool, error) {
	return s.res.familyAny(s.bind(ctx), id)
}

// ResolveHeaders answers a batch of addresses from headers only, gated and
// redacted, with the reader's world for bare ids. See
// [Resolver.ResolveHeaders].
func (s *ScriptReader) ResolveHeaders(ctx context.Context, refs []entity.Ref) map[entity.Ref]ResolvedHeader {
	return s.res.ResolveHeaders(s.bind(ctx), worldIn(ctx, s.world), refs)
}

// ListEntities yields only the entities the caller may read, redacted.
//
// Prefers ACL PUSHDOWN: when the gate can compose the caller's scope as a
// store predicate, the store never materializes rows the caller may not see
// and there is no per-type ReadableFacesMany probe. Field redaction still runs
// per yielded row — the pushdown replaces the row gate, not the field gate
// (RR-1W1G6K).
//
// Falls back to load-then-Filter when pushdown is unavailable (no
// ReadQueryProvider, a store without GraphQueryer, a type-less query, or a
// Reader that cannot redact without gating). The fallback is a performance
// regression only: both paths gate on the same policy.
func (s *ScriptReader) ListEntities(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	bound := s.bind(ctx)
	if red, ok := s.reader.(rowRedactor); ok {
		if seq, pushed := listPushdown(bound, s.provider, s.raw, red.RedactRow, q); pushed {
			return seq
		}
	}
	return func(yield func(*entity.Entity, error) bool) {
		var batch []*entity.Entity
		for e, err := range s.raw.ListEntities(bound, q) {
			if err != nil {
				yield(nil, err)
				return
			}
			batch = append(batch, e)
		}
		for _, e := range s.reader.Filter(bound, batch) {
			if !yield(e, nil) {
				return
			}
		}
	}
}

// headerGateChunk is how many headers ListEntityHeaders gates at once.
//
// The row gate is deliberately BATCHED — one ReadableFacesMany per distinct
// type per chunk rather than one probe per row — so gating cannot stream a
// row at a time without turning an O(types) probe count into O(rows). This
// chunk is the compromise: peak retention is bounded by the chunk, not by
// the store size, while the probe count stays proportional to
// rows/headerGateChunk.
//
// 512 is chosen so the probe overhead is amortized (a chunk holds many rows
// of each type in any realistic mix) while the retained slice stays small —
// headers carry no bodies, so 512 of them is tens of KB, not tens of MB.
const headerGateChunk = 512

// ListEntityHeaders yields content-free headers the caller may read,
// redacted — the header analog of [ScriptReader.ListEntities].
//
// Bounded in BOTH dimensions, which is the whole point (TKT-1ESTYJ):
//
//   - Per row, no body is carried. Backends that can project the content
//     column away never read it (see [store.ListEntityHeaders]).
//   - Across rows, at most [headerGateChunk] headers are retained at once.
//     ListEntities cannot make this promise: it materializes the ENTIRE
//     matching set to hand to Filter, so a whole-store scan retains a
//     whole store. That buffer — not the analyzers' own slices — is why a
//     type-less analyze scan held ~1 GB of markdown.
//
// Gating is identical to ListEntities': the same Reader, hence the same
// policy, on the same (id, type) pairs. Chunking changes only WHEN probes
// happen, never their verdicts, because a row's visibility does not depend
// on which other rows accompany it.
//
// Falls back to whole-entity load-then-Filter when the Reader cannot filter
// headers, so an incomplete Reader loses the memory win but never the gate.
func (s *ScriptReader) ListEntityHeaders(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[store.EntityHeader, error] {
	hf, ok := s.reader.(HeaderFilterer)
	if !ok {
		return s.headersViaEntities(ctx, q)
	}
	bound := s.bind(ctx)
	return func(yield func(store.EntityHeader, error) bool) {
		batch := make([]store.EntityHeader, 0, headerGateChunk)
		flush := func() bool {
			for _, h := range hf.FilterHeaders(bound, batch) {
				if !yield(h, nil) {
					return false
				}
			}
			batch = batch[:0]
			return true
		}
		for h, err := range store.ListEntityHeaders(bound, s.raw, q) {
			if err != nil {
				yield(store.EntityHeader{}, err)
				return
			}
			batch = append(batch, h)
			if len(batch) < headerGateChunk {
				continue
			}
			if !flush() {
				return
			}
		}
		if len(batch) > 0 {
			_ = flush()
		}
	}
}

// headersViaEntities is the fallback for a [Reader] without
// [HeaderFilterer]: gate whole entities through ListEntities, then project.
// Correct but unbounded — it inherits ListEntities' full-set buffering — so
// it trades the memory win for gate parity, never the reverse.
func (s *ScriptReader) headersViaEntities(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[store.EntityHeader, error] {
	return func(yield func(store.EntityHeader, error) bool) {
		for e, err := range s.ListEntities(ctx, q) {
			if err != nil {
				yield(store.EntityHeader{}, err)
				return
			}
			if !yield(store.HeaderOf(e), nil) {
				return
			}
		}
	}
}

// ListRelations yields only relations whose BOTH endpoints are visible
// (FROM ∧ TO). An explicit From/To filter in q therefore does not guarantee
// the row survives — see the binding's comment (RR-7GDT1Y).
func (s *ScriptReader) ListRelations(
	ctx context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	bound := s.bind(ctx)
	return func(yield func(*entity.Relation, error) bool) {
		var batch []*entity.Relation
		for rel, err := range s.raw.ListRelations(bound, q) {
			if err != nil {
				yield(nil, err)
				return
			}
			batch = append(batch, rel)
		}
		for _, rel := range s.reader.FilterRelations(bound, batch) {
			if !yield(rel, nil) {
				return
			}
		}
	}
}

// ListRelationsStrict is [ScriptReader.ListRelations] for an aggregate: a
// gate fault is yielded as an error instead of hiding the relations it
// touches (see [Reader.FilterRelationsStrict]).
func (s *ScriptReader) ListRelationsStrict(
	ctx context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	bound := s.bind(ctx)
	return func(yield func(*entity.Relation, error) bool) {
		var batch []*entity.Relation
		for rel, err := range s.raw.ListRelations(bound, q) {
			if err != nil {
				yield(nil, err)
				return
			}
			batch = append(batch, rel)
		}
		kept, err := s.reader.FilterRelationsStrict(bound, batch)
		if err != nil {
			yield(nil, err)
			return
		}
		for _, rel := range kept {
			if !yield(rel, nil) {
				return
			}
		}
	}
}
