package sqlchannels

import (
	"context"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// getChannelsSQL is the first GetMessages statement for n keys: the id of
// each live channel, four text columns per key, as readsSQL matches them.
func getChannelsSQL(n int) string {
	return `SELECT id, anchor_id, anchor_face, type, name FROM channels
	WHERE tomb = '' AND (anchor_id, anchor_face, type, name) IN (VALUES (?, ?, ?, ?)` +
		strings.Repeat(", (?, ?, ?, ?)", n-1) + `)`
}

// getMessagesSQL is the second GetMessages statement for n refs: a channel
// id and a seq per ref. Both are cast, so postgres does not read the
// VALUES parameters as text.
func getMessagesSQL(n int) string {
	const pair = `(CAST(? AS BIGINT), CAST(? AS BIGINT))`
	return `SELECT channel_id, ` + msgCols + ` FROM channel_messages
	WHERE (channel_id, seq) IN (VALUES ` + pair + strings.Repeat(", "+pair, n-1) + `)`
}

// GetMessagesSQL returns the two GetMessages statements, with `?`
// placeholders, for nKeys channels and nRefs refs, so the query-plan tests
// of the SQL backends explain the production text.
func GetMessagesSQL(nKeys, nRefs int) (chans, msgs string) {
	return getChannelsSQL(nKeys), getMessagesSQL(nRefs)
}

// GetMessages implements [channels.Store]. It makes two statements: the
// ids of the named live channels by their key columns, then the messages
// by (channel id, seq) on the primary key. A message whose doc does not
// decode is skipped with a warning.
func (s *Store) GetMessages(
	ctx context.Context, refs []channels.MessageRef,
) (map[channels.MessageRef]channels.Message, error) {
	if err := channels.CheckGetMessages(refs); err != nil {
		return nil, err
	}
	out := make(map[channels.MessageRef]channels.Message, len(refs))
	keys, seqs := channels.GroupRefs(refs)
	if len(keys) == 0 {
		return out, nil
	}
	args := make([]any, 0, 4*len(keys))
	for _, k := range keys {
		args = append(args, k.ID, string(k.Face), k.Type, k.Name)
	}
	ids, err := collect(ctx, s.db, getChannelsSQL(len(keys)), args, scanChannelID)
	if err != nil {
		return nil, s.wrap(err, "read channels of messages")
	}
	if len(ids) == 0 {
		return out, nil
	}
	byID := make(map[int64]channels.Key, len(ids))
	args = args[:0]
	for _, c := range ids {
		byID[c.id] = c.key
		for _, seq := range seqs[c.key] {
			args = append(args, c.id, seq)
		}
	}
	rows, err := collect(ctx, s.db, getMessagesSQL(len(args)/2), args, scanRefMessage)
	if err != nil {
		return nil, s.wrap(err, "read messages")
	}
	now := time.Now()
	for _, r := range rows {
		k := byID[r.channelID]
		if r.damaged != nil {
			s.damaged.Warn(k, now, s.d.Name+": skipping a message whose doc is damaged", r.damaged)
			continue
		}
		out[channels.MessageRef{Key: k, Seq: r.msg.Seq}] = r.msg
	}
	return out, nil
}

// channelID is a row of the first GetMessages statement.
type channelID struct {
	id  int64
	key channels.Key
}

func scanChannelID(r interface{ Scan(...any) error }) (channelID, error) {
	var out channelID
	var face string
	if err := r.Scan(&out.id, &out.key.ID, &face, &out.key.Type, &out.key.Name); err != nil {
		return out, err
	}
	out.key.Face = entity.Face(face)
	return out, nil
}

// refMessage is a row of the second GetMessages statement. damaged is set
// when the message's doc does not decode.
type refMessage struct {
	channelID int64
	msg       channels.Message
	damaged   error
}

func scanRefMessage(r interface{ Scan(...any) error }) (refMessage, error) {
	var out refMessage
	var m channels.Message
	var c msgScan
	if err := r.Scan(append([]any{&out.channelID}, c.dest(&m)...)...); err != nil {
		return out, err
	}
	out.msg, out.damaged = c.decode(m)
	return out, nil
}
