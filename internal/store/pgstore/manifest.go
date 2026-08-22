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
			-- pointer = '' / from_pointer = '': FAMILY-SCOPED. Sync is a
			-- DEFAULT-WORLD protocol (TKT-DOFYR1) and stays one; it is not
			-- a world scope and gains no world arm (TKT-WAV8XP PR-C).
			-- LOCKSTEP with the deletions arm below, whose equivalent test
			-- is spelled as an @-absence test via strpos, and is therefore
			-- INVISIBLE to a pointer grep — change one, change all three.
			SELECT 'e' AS kind, id AS a, '' AS b, '' AS c, type AS typ, false AS deleted, seq
			FROM entities WHERE pointer = ''
			UNION ALL
			SELECT 'r', from_id, rel_type, to_id, '', false, seq
			FROM relations WHERE from_pointer = ''
			UNION ALL
			-- id_a carrying '@' is a state tombstone (the id grammar forbids
			-- '@'); sync is a DEFAULT-WORLD protocol in Step 1 (TKT-DOFYR1) —
			-- an id-keyed peer cannot apply or delete a state, so state rows
			-- and tombstones stay out of the manifest entirely.
			--
			-- TRAP (TKT-WAV8XP PR-C, RULING 4): this is the SAME
			-- default-pointer test as the two arms above, expressed as
			-- '@'-ABSENCE over a codec'd state ref rather than as
			-- an equality test. A regex sweep for pointer MISSES IT
			-- SILENTLY. It is family-scoped, not a world scope, and must
			-- stay in lockstep with lines 30/33 — a sweep that worlds
			-- those two and skips this one would put state tombstones
			-- into an id-keyed sync stream.
			SELECT kind, id_a, id_b, id_c, typ, true, seq FROM deletions
			WHERE strpos(id_a, '@') = 0
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
