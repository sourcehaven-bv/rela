package sqlchannels

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// capRowsSQL reads one target's rows in one channel, oldest first: at most
// [channels.MaxAddressedPerTarget] entries of channel_mentions_cap_idx.
const capRowsSQL = `SELECT seq, kind, author_user FROM channel_mentions
	WHERE channel_id = ? AND target = ? ORDER BY seq`

// addressedSQL is the ListAddressed statement for every kind: target,
// reader user, reader org and limit. addressedKindSQL is the statement
// for one kind: target, kind, reader user, reader org and limit. The inner
// scan reads at most [channels.MaxAddressedScan] entries, of
// channel_mentions_target_idx or channel_mentions_kind_idx, in index
// order; nothing joins channel_messages, so no document is decoded.
var (
	addressedSQL     = addressedListSQL("")
	addressedKindSQL = addressedListSQL(" AND kind = ?")
)

// addressedListSQL is the ListAddressed statement with kindFilter added to
// the inner scan's WHERE.
func addressedListSQL(kindFilter string) string {
	return `SELECT ` + chanCols + `, a.seq, a.at, a.kind, a.author_user, a.author_tool,
	a.author_org, a.author_type, a.msg_type, a.reply_to
	FROM (SELECT channel_id, seq, kind, at, author_user, author_tool, author_org, author_type, msg_type, reply_to
		FROM channel_mentions WHERE target = ?` + kindFilter + `
		ORDER BY at DESC, channel_id DESC, seq DESC LIMIT ` + strconv.Itoa(channels.MaxAddressedScan) + `) a
	JOIN channels c ON c.id = a.channel_id AND c.tomb = ''
	LEFT JOIN channel_reads r ON r.channel_id = a.channel_id AND r.reader_user = ? AND r.reader_org = ?
	WHERE r.seq IS NULL OR r.seq < a.seq
	ORDER BY a.at DESC, c.anchor_id, c.anchor_face, c.type, c.name, a.seq, a.kind
	LIMIT ?`
}

// AddressedStatements are the addressed-row statements as the store runs
// them, with `?` placeholders, so the query-plan tests of the SQL backends
// explain the production text. List takes target, reader user, reader org
// and limit; ListKind takes target, kind, reader user, reader org and
// limit; Cap takes channel id and target.
type AddressedStatements struct {
	List, ListKind, Cap string
}

// AddressedSQL returns the [AddressedStatements].
func AddressedSQL() AddressedStatements {
	return AddressedStatements{List: addressedSQL, ListKind: addressedKindSQL, Cap: capRowsSQL}
}

// writeAddressed writes the rows of message m in channel r: each through
// the cap step, then one multi-row insert. It reports whether a reply row
// was written. Rows that exist already are kept as they are, with their
// time.
func writeAddressed(
	ctx context.Context, q Querier, r row, m channels.Message, rows []channels.AddressedRow, at time.Time,
) (replyWritten bool, err error) {
	keep := make([]channels.AddressedRow, 0, len(rows))
	byTarget := make(map[string][]channels.AddressedRow)
	var order []string
	for _, nr := range rows {
		if _, ok := byTarget[nr.Target]; !ok {
			order = append(order, nr.Target)
		}
		byTarget[nr.Target] = append(byTarget[nr.Target], nr)
	}
	for _, target := range order {
		n, stepErr := capStep(ctx, q, r.id, target, len(byTarget[target]), m.Author.User)
		if stepErr != nil {
			return false, stepErr
		}
		for i, nr := range byTarget[target] {
			if i >= n {
				channels.LogSkippedRow(r.c.Key, m.Seq, target)
				continue
			}
			keep = append(keep, nr)
		}
	}
	if len(keep) == 0 {
		return false, nil
	}
	args := make([]any, 0, 11*len(keep))
	for _, nr := range keep {
		replyWritten = replyWritten || nr.Kind == channels.AddressReply
		args = append(args, r.id, m.Seq, nr.Target, string(nr.Kind), micros(at), m.Author.User, m.Author.Tool,
			m.Author.OrgID, m.Author.PrincipalType, m.Type, m.ReplyTo)
	}
	_, err = q.Exec(ctx, `INSERT INTO channel_mentions
		(channel_id, seq, target, kind, at, author_user, author_tool, author_org, author_type, msg_type, reply_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`+strings.Repeat(", (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", len(keep)-1)+
		` ON CONFLICT DO NOTHING`, args...)
	return replyWritten, err
}

// capStep makes room for want new rows of target in channel chanID by
// author, as [channels.Evict] picks, and returns how many of them may be
// written.
func capStep(ctx context.Context, q Querier, chanID int64, target string, want int, author string) (int, error) {
	held, err := collect(ctx, q, capRowsSQL, []any{chanID, target},
		func(s interface{ Scan(...any) error }) (channels.AddressedRow, error) {
			out := channels.AddressedRow{Target: target}
			var kind string
			err := s.Scan(&out.Seq, &kind, &out.Author)
			out.Kind = channels.AddressKind(kind)
			return out, err
		})
	if err != nil {
		return 0, err
	}
	free := channels.MaxAddressedPerTarget - len(held)
	if free >= want {
		return want, nil
	}
	var markers []int64
	readMarkers := func(string) []int64 { return markers }
	if markers, err = collect(ctx, q, `SELECT seq FROM channel_reads WHERE channel_id = ? AND reader_user = ?`,
		[]any{chanID, target}, func(s interface{ Scan(...any) error }) (int64, error) {
			var v int64
			return v, s.Scan(&v)
		}); err != nil {
		return 0, err
	}
	for free < want {
		i, ok := channels.Evict(held, readMarkers, author)
		if !ok {
			break
		}
		ev := held[i]
		if _, err := q.Exec(ctx, `DELETE FROM channel_mentions WHERE channel_id = ? AND seq = ? AND target = ?
			AND kind = ?`, chanID, ev.Seq, target, string(ev.Kind)); err != nil {
			return 0, err
		}
		held = append(held[:i], held[i+1:]...)
		free++
	}
	return max(free, 0), nil
}

// editAddressed replaces the mention rows of message m, whose targets were
// old, with the set m.Mentions: rows of dropped targets go, rows of added
// targets are written at at, kept rows keep their time. The reply row
// stays.
func editAddressed(ctx context.Context, q Querier, r row, m channels.Message, old []channels.Mention) error {
	targets := channels.MentionTargets(m.Mentions)
	query := `DELETE FROM channel_mentions WHERE channel_id = ? AND seq = ? AND kind = 'mention'`
	args := []any{r.id, m.Seq}
	if len(targets) > 0 {
		query += ` AND target NOT IN (?` + strings.Repeat(", ?", len(targets)-1) + `)`
		for _, t := range targets {
			args = append(args, t)
		}
	}
	if _, err := q.Exec(ctx, query, args...); err != nil {
		return err
	}
	added := channels.Message{Seq: m.Seq, Author: m.Author, Mentions: channels.AddedMentions(old, m.Mentions)}
	_, err := writeAddressed(ctx, q, r, m, channels.AppendRows(added, ""), m.EditedAt)
	return err
}

// removeAddressed deletes every row of message seq in channel chanID.
func removeAddressed(ctx context.Context, q Querier, chanID, seq int64) error {
	_, err := q.Exec(ctx, `DELETE FROM channel_mentions WHERE channel_id = ? AND seq = ?`, chanID, seq)
	return err
}

// ListAddressed implements [channels.Store]. A damaged channel row is
// skipped with a warning, as ListReads does.
func (s *Store) ListAddressed(
	ctx context.Context, rd channels.Reader, kind channels.AddressKind, limit int,
) ([]channels.Addressed, error) {
	if err := channels.CheckListAddressed(rd, kind, limit); err != nil {
		return nil, err
	}
	query, args := addressedSQL, []any{rd.User, rd.User, rd.OrgID, limit}
	if kind != channels.AddressAny {
		query, args = addressedKindSQL, []any{rd.User, string(kind), rd.User, rd.OrgID, limit}
	}
	rows, err := collect(ctx, s.db, query, args, scanAddressed)
	if err != nil {
		return nil, s.wrap(err, "list addressed rows")
	}
	out := make([]channels.Addressed, 0, len(rows))
	now := time.Now()
	for _, r := range rows {
		if r.damaged != nil {
			s.damaged.Warn(r.damaged.key, now, s.d.Name+": skipping an addressed row whose channel row is damaged",
				r.damaged.err)
			continue
		}
		out = append(out, r.row)
	}
	return out, nil
}

// addressedRow is a row of [Store.ListAddressed]: an addressed row, or the
// damage that keeps its channel from decoding.
type addressedRow struct {
	row     channels.Addressed
	damaged *damagedRowError
}

func scanAddressed(r interface{ Scan(...any) error }) (addressedRow, error) {
	var a channels.Addressed
	var at int64
	var kind string
	v, err := scanChannelWith(r, &a.Seq, &at, &kind, &a.Author.User, &a.Author.Tool, &a.Author.OrgID,
		&a.Author.PrincipalType, &a.Type, &a.ReplyTo)
	if d, ok := errors.AsType[*damagedRowError](err); ok {
		return addressedRow{damaged: d}, nil
	}
	if err != nil {
		return addressedRow{}, err
	}
	a.Channel, a.At, a.Kind = v.c, fromMicros(at), channels.AddressKind(kind)
	return addressedRow{row: a}, nil
}
