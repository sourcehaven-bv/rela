package visibility

import (
	"context"
	"errors"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ScriptReader adapts a [Reader] to the three-method read surface script
// runtimes consume (internal/lua's EntityReader, satisfied structurally so
// lua needs no dependency on this package). Every method row-gates and
// field-redacts through the wrapped Reader, so a script sees exactly the
// caller's view (DEC-O59WM4).
//
// # Why load-then-Filter rather than Reader.Get
//
// [Reader.Get] gates BEFORE the load and therefore needs the entity type up
// front, but a script asks by bare ID (`rela.get_entity(id)`). Rather than
// invent a type claim — which would reintroduce the BUG-ZWTDH9 cross-type
// surface [Reader.Get] exists to close — ScriptReader loads raw and then
// runs the result through [Reader.Filter], which gates on the entity's
// STORED type. A denied entity comes back as a not-found error, which is
// what the bindings already translate into nil.
//
// # Allocation
//
// ListEntities materializes the matching set to filter it (the row gate is
// batched by design — one probe per distinct type rather than one per row).
// `rela.list_entities` has no limit today, so on a large type this is
// roughly 3x the peak allocation of an ungated stream: the slice, the
// filtered slice, and the redacted copies.
//
// Two costs the batching does NOT amortize (RR-3U1V80): the field redactor
// runs PER ROW (one verdict resolution each), and FilterRelations performs
// one entity load PER DISTINCT ENDPOINT — so a wide `get_relations` is an
// N-load amplification, not just an N-allocation one. Bounding these
// bindings is TKT-YWDGZD; this type does not paper over them.
type ScriptReader struct {
	reader Reader
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
// their properties) the caller may see.
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
	s := &ScriptReader{reader: reader, raw: raw, binder: binder}
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

// GetEntity loads by ID and gates on the STORED type. A denied entity is
// reported as [store.ErrNotFound] — indistinguishable from a genuine miss,
// preserving the oracle-free contract the rest of the package keeps.
func (s *ScriptReader) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	ctx = s.bind(ctx)
	e, err := s.raw.GetEntity(ctx, id)
	if err != nil {
		return nil, err
	}
	visible := s.reader.Filter(ctx, []*entity.Entity{e})
	if len(visible) == 0 {
		return nil, store.ErrNotFound
	}
	return visible[0], nil
}

// ListEntities yields only the entities the caller may read, redacted.
//
// Prefers ACL PUSHDOWN: when the gate can compose the caller's scope as a
// store predicate, the store never materializes rows the caller may not see
// and there is no per-type PermitsReadMany probe. Field redaction still runs
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
