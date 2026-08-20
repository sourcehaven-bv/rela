package pgstore

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// writeEntityTombstone records an entity deletion in the deletions table within
// the supplied transaction, so the tombstone is atomic with the DELETE. The
// type is preserved so a catch-up/manifest can emit a structurally complete
// EventEntityDeleted.
// The id_a slot carries the STATE reference in its boundary
// serialization ("PAGE-1" or "PAGE-1@draft", TKT-DOFYR1) — tombstones
// are a wire/manifest boundary, so the codec form is the contract;
// readers parse it back through entity.ParseStateRef.
func (s *Store) writeEntityTombstone(
	ctx context.Context, q DBTX, id string, p entity.Pointer, entityType string,
) error {
	const ins = `INSERT INTO deletions (kind, id_a, typ) VALUES ('e', $1, $2)`
	_, err := q.Exec(ctx, ins, entity.FormatStateRef(id, p), entityType)
	return err
}

// writeRelationTombstone records a relation deletion in the deletions table
// within the supplied transaction. Relations carry no type, so typ is left
// empty; the from/rel_type/to triple lands in id_a/id_b/id_c.
// The id_a (from) slot carries the tail-state reference via the codec,
// like the entity tombstone above.
func (s *Store) writeRelationTombstone(
	ctx context.Context, q DBTX, from string, fp entity.Pointer, relType, to string,
) error {
	const ins = `INSERT INTO deletions (kind, id_a, id_b, id_c) VALUES ('r', $1, $2, $3)`
	_, err := q.Exec(ctx, ins, entity.FormatStateRef(from, fp), relType, to)
	return err
}

// writeTombstonesForEvents records a tombstone for each delete event in evs,
// within the supplied transaction. Non-delete events are ignored. This keeps
// the tombstone set in lock-step with the events the delete path already builds.
func (s *Store) writeTombstonesForEvents(ctx context.Context, q DBTX, evs []store.Event) error {
	for _, ev := range evs {
		//exhaustive:ignore // only delete events produce tombstones; create/update are intentionally skipped.
		switch ev.Op {
		case store.EventEntityDeleted:
			if err := s.writeEntityTombstone(ctx, q, ev.EntityID, ev.Pointer, ev.EntityType); err != nil {
				return err
			}
		case store.EventRelationDeleted:
			if err := s.writeRelationTombstone(ctx, q, ev.From, ev.Pointer, ev.RelationType, ev.To); err != nil {
				return err
			}
		}
	}
	return nil
}
