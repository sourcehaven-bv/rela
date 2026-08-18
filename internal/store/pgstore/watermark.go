package pgstore

import (
	"context"
	"fmt"
)

// EntityTypeWatermark implements [store.TypeWatermark].
//
// One index-only query over two seq indexes, replacing a full render of the
// collection. Both halves are required:
//
//   - live rows give the newest create/update/rename for the type;
//   - deletion tombstones give the newest REMOVAL, which the live table cannot
//     report because deletes are hard (see migrations/0003_sync.sql). Without
//     them, deleting the newest row lowers max(seq) and the watermark goes
//     BACKWARDS — a client that already saw the higher value would never poll
//     again.
//
// The tombstone half is scoped by type alone. A tombstone records only
// (kind, id_a, typ), so a narrower predicate cannot be applied to a row that no
// longer exists; see the [store.TypeWatermark] doc for why over-triggering is
// the accepted, safe direction.
//
// COALESCE, not GREATEST-of-two-subqueries: an empty table yields NULL, and
// GREATEST(NULL, 5) is NULL in PostgreSQL — which would silently report 0 for a
// type whose entities all still exist but which has never had a deletion.
func (s *Store) EntityTypeWatermark(ctx context.Context, entityType string) (int64, error) {
	const q = `
		SELECT COALESCE(MAX(seq), 0) FROM (
			SELECT seq FROM entities  WHERE type = $1
			UNION ALL
			SELECT seq FROM deletions WHERE kind = 'e' AND typ = $1
		) t`
	var seq int64
	if err := s.db.QueryRow(ctx, q, entityType).Scan(&seq); err != nil {
		return 0, fmt.Errorf("pgstore: entity type watermark for %q: %w", entityType, err)
	}
	return seq, nil
}
