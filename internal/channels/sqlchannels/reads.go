package sqlchannels

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// readsChunk is how many keys one Reads statement names, for the reason
// waitingChunk gives: 500 four-column rows stay far below either
// database's variable limit.
const readsChunk = 500

// unreadSQL counts the counting messages above the marker r of channel
// r.channel_id, stopping at [channels.UnreadCap]. The literal `counts = 1`
// matches the predicate of the partial index channel_messages_counts_idx,
// so each count is a range of at most UnreadCap index entries.
var unreadSQL = `(SELECT COUNT(*) FROM (SELECT 1 FROM channel_messages m
	WHERE m.channel_id = r.channel_id AND m.seq > r.seq AND m.counts = 1 LIMIT ` +
	strconv.Itoa(channels.UnreadCap) + `) u)`

// markStateSQL reads one marker with its unread count: SetRead's answer.
var markStateSQL = `SELECT r.seq, r.at, ` + unreadSQL + ` FROM channel_reads r
	WHERE r.channel_id = ? AND r.reader_user = ? AND r.reader_org = ?`

// readsSQL is the Reads statement for n keys: the reader, then four
// columns per key.
func readsSQL(n int) string {
	return `SELECT c.anchor_id, c.anchor_face, c.type, c.name, c.last_seq,
	r.seq, r.at, ` + unreadSQL + ` FROM channels c JOIN channel_reads r ON r.channel_id = c.id
	WHERE c.tomb = '' AND r.reader_user = ? AND r.reader_org = ?
	AND (c.anchor_id, c.anchor_face, c.type, c.name) IN (VALUES (?, ?, ?, ?)` +
		strings.Repeat(", (?, ?, ?, ?)", n-1) + `)`
}

// windowSQL is the ListReads statement: reader user, org and limit.
var windowSQL = `SELECT ` + chanCols + `, c.last_at, r.seq, r.at
	FROM channel_reads r JOIN channels c ON c.id = r.channel_id
	WHERE r.reader_user = ? AND r.reader_org = ? AND c.tomb = ''
	ORDER BY (c.last_seq > r.seq) DESC, c.last_at DESC, c.anchor_id, c.anchor_face, c.type, c.name
	LIMIT ?`

// ReadStatements are the read marker statements as the store runs them,
// with `?` placeholders, so the query-plan tests of the SQL backends
// explain the production text. Mark takes channel id, user and org; Reads
// takes user and org, then four columns for each of nKeys keys; Window
// takes user, org and limit.
type ReadStatements struct {
	Mark, Reads, Window string
}

// ReadSQL returns the [ReadStatements] with nKeys keys in Reads.
func ReadSQL(nKeys int) ReadStatements {
	return ReadStatements{Mark: markStateSQL, Reads: readsSQL(nKeys), Window: windowSQL}
}

// upsertRead creates the reader's marker on channel chanID at seq, or
// raises it to seq. It never lowers a marker, and it sets at only when the
// marker moves. It returns the rows it changed: 1 when the marker was
// created or raised, 0 otherwise.
func upsertRead(
	ctx context.Context, q Querier, chanID int64, r channels.Reader, seq int64, at time.Time,
) (int64, error) {
	return q.Exec(ctx, `INSERT INTO channel_reads (channel_id, reader_user, reader_org, seq, at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (channel_id, reader_user, reader_org)
		DO UPDATE SET seq = excluded.seq, at = excluded.at WHERE excluded.seq > channel_reads.seq`,
		chanID, r.User, r.OrgID, seq, micros(channels.NormTime(at)))
}

// SetRead implements [channels.Store]. It runs under the entity lock, so
// it serializes with a Rekey of the anchor, and reads the channel row,
// moves the marker and counts in one transaction.
func (s *Store) SetRead(
	ctx context.Context, k channels.Key, rd channels.Reader, seq int64, at time.Time,
) (channels.ReadState, bool, error) {
	if err := channels.CheckSetRead(k, rd, seq); err != nil {
		return channels.ReadState{}, false, err
	}
	var out channels.ReadState
	var moved bool
	werr := s.write(ctx, []string{k.ID}, func(q Querier) error {
		r, err := s.channelRow(ctx, q, k)
		if err != nil {
			return err
		}
		n, err := upsertRead(ctx, q, r.id, rd, min(seq, r.c.LastSeq), at)
		if err != nil {
			return err
		}
		moved = n > 0
		var markAt int64
		if err := q.QueryRow(ctx, markStateSQL, r.id, rd.User, rd.OrgID).
			Scan(&out.Seq, &markAt, &out.Unread); err != nil {
			return err
		}
		out.At, out.LastSeq = fromMicros(markAt), r.c.LastSeq
		return nil
	})
	return out, moved, s.wrap(werr, "mark %s read", k)
}

// Reads implements [channels.Store]. It runs one statement per chunk of
// [readsChunk] keys.
func (s *Store) Reads(
	ctx context.Context, rd channels.Reader, keys []channels.Key,
) (map[channels.Key]channels.ReadState, error) {
	if err := channels.CheckReads(rd, keys); err != nil {
		return nil, err
	}
	out := make(map[channels.Key]channels.ReadState, len(keys))
	for chunk := range slices.Chunk(keys, readsChunk) {
		args := make([]any, 0, 2+4*len(chunk))
		args = append(args, rd.User, rd.OrgID)
		for _, k := range chunk {
			args = append(args, k.ID, string(k.Face), k.Type, k.Name)
		}
		rows, err := collect(ctx, s.db, readsSQL(len(chunk)), args, scanReadState)
		if err != nil {
			return nil, s.wrap(err, "read markers")
		}
		for _, r := range rows {
			out[r.key] = r.state
		}
	}
	return out, nil
}

// readRow is a row of [Store.Reads].
type readRow struct {
	key   channels.Key
	state channels.ReadState
}

func scanReadState(r interface{ Scan(...any) error }) (readRow, error) {
	var out readRow
	var face string
	var at int64
	if err := r.Scan(&out.key.ID, &face, &out.key.Type, &out.key.Name, &out.state.LastSeq,
		&out.state.Seq, &at, &out.state.Unread); err != nil {
		return out, err
	}
	out.key.Face = entity.Face(face)
	out.state.At = fromMicros(at)
	return out, nil
}

// ListReads implements [channels.Store]. channel_reads_reader_idx finds
// the reader's markers; the sort runs over those only. Channels with
// messages above the marker sort first, so a channel whose marker is old
// is never cut while it has news. A damaged channel row is skipped with a
// warning, as ListSessionsDue does.
func (s *Store) ListReads(ctx context.Context, rd channels.Reader, limit int) ([]channels.ReadMarker, error) {
	if err := channels.CheckListReads(rd, limit); err != nil {
		return nil, err
	}
	rows, err := collect(ctx, s.db, windowSQL, []any{rd.User, rd.OrgID, limit}, scanMarker)
	if err != nil {
		return nil, s.wrap(err, "list read markers")
	}
	out := make([]channels.ReadMarker, 0, len(rows))
	now := time.Now()
	for _, r := range rows {
		if r.damaged != nil {
			s.damaged.Warn(r.damaged.key, now, s.d.Name+": skipping a read marker whose channel row is damaged",
				r.damaged.err)
			continue
		}
		out = append(out, r.marker)
	}
	return out, nil
}

// markerRow is a row of [Store.ListReads]: a marker, or the damage that
// keeps its channel from decoding.
type markerRow struct {
	marker  channels.ReadMarker
	damaged *damagedRowError
}

func scanMarker(r interface{ Scan(...any) error }) (markerRow, error) {
	var lastAt, seq, at int64
	v, err := scanChannelWith(r, &lastAt, &seq, &at)
	if d, ok := errors.AsType[*damagedRowError](err); ok {
		return markerRow{damaged: d}, nil
	}
	if err != nil {
		return markerRow{}, err
	}
	return markerRow{marker: channels.ReadMarker{
		Channel: v.c, Seq: seq, At: fromMicros(at), LastAt: fromMicros(lastAt),
	}}, nil
}
