package pgstore

import (
	"context"
	"fmt"

	synctypes "github.com/Sourcehaven-BV/rela/internal/sync"
)

// ManifestSince returns every change with seq > cursor, in seq order: live
// entity/relation rows (Deleted=false) UNION deletion tombstones (Deleted=true).
//
// A deleted-then-recreated record yields two entries — the tombstone and the new
// live row — each with its own seq, so a client that was behind both sees the
// net effect in order. Callers advance their cursor to the Seq of the last entry
// they processed. The scan is an index range over the seq indexes added in
// migration 0003.
//
// RETENTION CAVEAT: the deletions table grows without bound (every delete ever
// adds a tombstone; nothing prunes today), so ManifestSince(0) replays the
// entire deletion history and returns the full result set in one slice. A fresh
// client should bootstrap from a full export and only then track the cursor,
// rather than rely on cursor 0 over a long-lived churny dataset. Tombstone
// pruning (retention horizon) and manifest pagination (LIMIT + next-cursor) are
// documented follow-ups (see TKT-GFJJ3S notes), not built here.
func (s *Store) ManifestSince(ctx context.Context, cursor int64) ([]synctypes.ManifestEntry, error) {
	const q = `
		SELECT kind, a, b, c, typ, deleted, seq FROM (
			SELECT 'e' AS kind, id AS a, '' AS b, '' AS c, type AS typ, false AS deleted, seq FROM entities
			UNION ALL
			SELECT 'r', from_id, rel_type, to_id, '', false, seq FROM relations
			UNION ALL
			SELECT kind, id_a, id_b, id_c, typ, true, seq FROM deletions
		) t
		WHERE seq > $1
		ORDER BY seq`
	rows, err := s.db.Query(ctx, q, cursor)
	if err != nil {
		return nil, fmt.Errorf("pgstore: manifest query: %w", err)
	}
	defer rows.Close()

	var out []synctypes.ManifestEntry
	for rows.Next() {
		var e synctypes.ManifestEntry
		if err := rows.Scan(&e.Kind, &e.IDA, &e.IDB, &e.IDC, &e.Typ, &e.Deleted, &e.Seq); err != nil {
			return nil, fmt.Errorf("pgstore: manifest scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgstore: manifest rows: %w", err)
	}
	return out, nil
}
