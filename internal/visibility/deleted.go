package visibility

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// GateDeleted returns last, the raw final state of a face that a delete
// removed, as reader would have shown it to the ctx principal: row-gated,
// then redacted. ok is false when the principal may not read it.
//
// reader is one of the script readers: a [*ScriptReader] gates, an
// [*UnrestrictedReader] passes last through, and a [DenyReader] or any
// other value refuses. It is a function, not a reader method, so the
// readers' public API stays the read surface scripts use.
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
func GateDeleted(ctx context.Context, reader any, last *entity.Entity) (*entity.Entity, bool) {
	if last == nil {
		return nil, false
	}
	switch r := reader.(type) {
	case *ScriptReader:
		if r == nil || worldIn(ctx, r.world).denied {
			return nil, false
		}
		out := r.reader.Filter(r.bind(ctx), []*entity.Entity{last})
		if len(out) != 1 {
			return nil, false
		}
		return out[0], true
	case *UnrestrictedReader:
		return last, r != nil
	default:
		return nil, false
	}
}
