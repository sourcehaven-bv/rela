package visibility

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// GateDeleted returns last, the raw final state of a face that a delete
// removed, as the ctx principal would have read it: row-gated, then
// redacted. ok is false when the principal may not read it.
//
// The row gate runs against the store, where the row no longer is. A grant
// that applies to every row of the type still answers, since it needs no
// row. A grant that needs the stored row, such as a local role conferred by
// a relation, finds nothing and denies. A row stored again at the same ref,
// after a restore or under a reused id, would answer instead, so the caller
// must refuse a state whose ref is live again.
//
// It serves the `on.deleted` automation job (TKT-Q0AQ840), which runs after
// the delete and so cannot read the face through [ScriptReader.GetAddress].
func (s *ScriptReader) GateDeleted(ctx context.Context, last *entity.Entity) (*entity.Entity, bool) {
	if last == nil || worldIn(ctx, s.world).denied {
		return nil, false
	}
	out := s.reader.Filter(s.bind(ctx), []*entity.Entity{last})
	if len(out) != 1 {
		return nil, false
	}
	return out[0], true
}

// GateDeleted returns last unchanged: this reader is ungated. See
// [ScriptReader.GateDeleted].
func (*UnrestrictedReader) GateDeleted(_ context.Context, last *entity.Entity) (*entity.Entity, bool) {
	return last, last != nil
}

// GateDeleted refuses, like every read on this reader. See
// [ScriptReader.GateDeleted].
func (DenyReader) GateDeleted(context.Context, *entity.Entity) (*entity.Entity, bool) {
	return nil, false
}
