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
// ListEntities materializes the matching set to filter it (Filter is
// batched by design — one gate probe per distinct type rather than one per
// row). `rela.list_entities` has no limit today, so on a large type this is
// roughly 3x the peak allocation of an ungated stream: the slice, the
// filtered slice, and the redacted copies. Bounding that binding is
// TKT-YWDGZD; this type does not paper over it.
type ScriptReader struct {
	reader Reader
	raw    store.Store
}

// NewScriptReader wraps reader over the raw store. Both are required: raw
// supplies the underlying rows, reader decides which of them (and which of
// their properties) the caller may see.
func NewScriptReader(reader Reader, raw store.Store) (*ScriptReader, error) {
	if reader == nil {
		return nil, errors.New("visibility: NewScriptReader: reader must be non-nil")
	}
	if raw == nil {
		return nil, errors.New("visibility: NewScriptReader: raw store must be non-nil")
	}
	return &ScriptReader{reader: reader, raw: raw}, nil
}

// GetEntity loads by ID and gates on the STORED type. A denied entity is
// reported as [store.ErrNotFound] — indistinguishable from a genuine miss,
// preserving the oracle-free contract the rest of the package keeps.
func (s *ScriptReader) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
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
func (s *ScriptReader) ListEntities(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		var batch []*entity.Entity
		for e, err := range s.raw.ListEntities(ctx, q) {
			if err != nil {
				yield(nil, err)
				return
			}
			batch = append(batch, e)
		}
		for _, e := range s.reader.Filter(ctx, batch) {
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
	return func(yield func(*entity.Relation, error) bool) {
		var batch []*entity.Relation
		for rel, err := range s.raw.ListRelations(ctx, q) {
			if err != nil {
				yield(nil, err)
				return
			}
			batch = append(batch, rel)
		}
		for _, rel := range s.reader.FilterRelations(ctx, batch) {
			if !yield(rel, nil) {
				return
			}
		}
	}
}
