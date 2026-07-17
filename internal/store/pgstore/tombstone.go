package pgstore

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// writeEntityTombstone records an entity deletion in the deletions table within
// the supplied transaction, so the tombstone is atomic with the DELETE. The
// type is preserved so a catch-up/manifest can emit a structurally complete
// EventEntityDeleted.
func (s *Store) writeEntityTombstone(ctx context.Context, q DBTX, id, entityType string) error {
	const ins = `INSERT INTO deletions (kind, id_a, typ) VALUES ('e', $1, $2)`
	_, err := q.Exec(ctx, ins, id, entityType)
	return err
}

// writeRelationTombstone records a relation deletion in the deletions table
// within the supplied transaction. Relations carry no type, so typ is left
// empty; the from/rel_type/to triple lands in id_a/id_b/id_c.
func (s *Store) writeRelationTombstone(ctx context.Context, q DBTX, from, relType, to string) error {
	const ins = `INSERT INTO deletions (kind, id_a, id_b, id_c) VALUES ('r', $1, $2, $3)`
	_, err := q.Exec(ctx, ins, from, relType, to)
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
			if err := s.writeEntityTombstone(ctx, q, ev.EntityID, ev.EntityType); err != nil {
				return err
			}
		case store.EventRelationDeleted:
			if err := s.writeRelationTombstone(ctx, q, ev.From, ev.RelationType, ev.To); err != nil {
				return err
			}
		}
	}
	return nil
}
