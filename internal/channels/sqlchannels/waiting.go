package sqlchannels

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// waitingChunk is how many anchors one ListWaiting statement names. Both
// databases share one `?` SQL text and the dialect binds no arrays, so a
// long anchor list runs as several statements in one call; 500 pairs stay
// far below either database's variable limit.
const waitingChunk = 500

// ListWaiting implements [channels.Store]. The anchor branch runs one
// statement per chunk of anchors; the creator branch runs one statement
// over created_by_user, which channels_creator_idx serves (pinned by
// sqlitechannels' TestListWaiting_CreatorUsesIndex). Rows merge by
// key, so a channel both branches return counts once, and the ceiling is
// taken on the merged count. Each statement is bounded too, so none reads
// unbounded; a statement that fills its LIMIT is over the ceiling, even
// when some of its rows are damaged, because rows past the LIMIT were
// never read. A damaged row is skipped with a warning, as ListSessionsDue
// does.
func (s *Store) ListWaiting(ctx context.Context, q channels.WaitingQuery) ([]channels.Channel, error) {
	if err := channels.CheckWaitingQuery(q); err != nil {
		return nil, err
	}
	stateSQL, stateArgs := waitingStates(q.States)
	stateSQL = `(` + stateSQL + `)` + waitingNames(q.Names)
	found := map[channels.Key]channels.Channel{}
	ceiling := q.RowCeiling()
	run := func(where string, args []any) error {
		all := append(append([]any{string(channels.StateOpen)}, stateArgs...), args...)
		all = append(all, ceiling+1)
		rows, err := collect(ctx, s.db, `SELECT `+chanCols+` FROM channels
			WHERE tomb = '' AND state = ? AND `+stateSQL+` AND `+where+` LIMIT ?`, all, scanDue)
		if err != nil {
			return s.wrap(err, "list waiting sessions")
		}
		if len(rows) > ceiling {
			return channels.TooManyWaiting(q)
		}
		now := time.Now()
		for _, r := range rows {
			if r.damaged != nil {
				s.damaged.Warn(r.damaged.key, now, s.d.Name+": skipping a waiting session whose row is damaged",
					r.damaged.err)
				continue
			}
			if _, dup := found[r.c.Key]; !dup && len(found) >= ceiling {
				return channels.TooManyWaiting(q)
			}
			found[r.c.Key] = r.c
		}
		return nil
	}
	for chunk := range slices.Chunk(q.Anchors, waitingChunk) {
		args := make([]any, 0, 2*len(chunk))
		for _, a := range chunk {
			args = append(args, a.ID, string(a.Face))
		}
		where := `(anchor_id, anchor_face) IN (VALUES (?, ?)` + strings.Repeat(", (?, ?)", len(chunk)-1) + `)`
		if err := run(where, args); err != nil {
			return nil, err
		}
	}
	if q.CreatorBranch() {
		args := []any{q.CreatedBy}
		for _, t := range q.CreatorTypes {
			args = append(args, t)
		}
		// The literal `<> ''` matches the predicate of the partial index
		// channels_creator_idx. A bound parameter cannot prove it, so
		// without it neither database uses the index.
		where := `created_by_user <> '' AND created_by_user = ? AND type IN (?` + strings.Repeat(", ?", len(q.CreatorTypes)-1) + `)`
		if err := run(where, args); err != nil {
			return nil, err
		}
	}
	out := make([]channels.Channel, 0, len(found))
	for _, c := range found {
		out = append(out, c)
	}
	channels.SortChannels(out)
	return out, nil
}

// waitingNames renders the name filter of a waiting query as a literal
// condition, empty for [channels.AnyName].
func waitingNames(f channels.NameFilter) string {
	switch f {
	case channels.UnnamedOnly:
		return ` AND name = ''`
	case channels.NamedOnly:
		return ` AND name <> ''`
	default:
		return ``
	}
}

// waitingStates renders the type and session state filter of a waiting
// query, types in sorted order so the SQL text is stable.
func waitingStates(states map[string][]string) (where string, args []any) {
	parts := make([]string, 0, len(states))
	for _, typ := range slices.Sorted(maps.Keys(states)) {
		list := states[typ]
		parts = append(parts, `(type = ? AND session_state IN (?`+strings.Repeat(", ?", len(list)-1)+`))`)
		args = append(args, typ)
		for _, st := range list {
			args = append(args, st)
		}
	}
	return strings.Join(parts, " OR "), args
}
