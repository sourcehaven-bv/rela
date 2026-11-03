package filechannels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// mentionsFile holds a channel's addressed rows, beside its meta.yaml and
// reads.json. It moves with the channel directory on a rekey, tombstone,
// untombstone and purge, and goes with it on a delete.
//
// The log and this file cannot be written in one step, so the store orders
// the writes inside the lock to keep the file from pointing at a message
// that does not address its target: rows that go are written before the
// log record (an edit's dropped targets, a removal), rows that come after
// it (an append, an edit's added targets). A failed write before the log
// record fails the operation; a failed write after it is logged and
// swallowed, because the message is already durable. An append writes
// the rows of [channels.Message.Mentions] and the reply row; an edit
// writes only the rows of the targets it added, so a row the cap evicted
// does not come back. A crash loses a notification; it never invents one.
const mentionsFile = "mentions.json"

// mentionsVersion is the format of mentions.json.
const mentionsVersion = 1

// mentionsDoc is mentions.json: the rows sorted by seq, target and kind.
type mentionsDoc struct {
	V    int          `json:"v"`
	Rows []mentionRow `json:"rows"`
}

// mentionRow is one addressed row in mentions.json.
type mentionRow struct {
	Seq     int64                `json:"seq"`
	Target  string               `json:"target"`
	Kind    channels.AddressKind `json:"kind"`
	At      time.Time            `json:"at"`
	Author  channels.Author      `json:"author"`
	Type    string               `json:"type"`
	ReplyTo int64                `json:"reply_to,omitempty"`
}

func mentionsKey(k channels.Key) string { return chanDir(k) + "/" + mentionsFile }

func (r mentionRow) capRow() channels.AddressedRow {
	return channels.AddressedRow{Seq: r.Seq, Target: r.Target, Kind: r.Kind, Author: r.Author.User}
}

// readRows returns the addressed rows of channel k, from the channel
// directory dir. A missing file holds none. A damaged file reads as none
// too, with a warning: the next row write replaces it.
//
// Nil: returned when the channel holds no rows.
func readRows(d *disk, dir string, k channels.Key) ([]mentionRow, error) {
	data, err := d.readFile(dir + "/" + mentionsFile)
	if err != nil {
		return nil, fmt.Errorf("filechannels: read addressed rows of %s: %w", k, err)
	}
	if data == nil {
		return nil, nil
	}
	var doc mentionsDoc
	if err := json.Unmarshal(data, &doc); err != nil || doc.V != mentionsVersion {
		d.damaged.Warn(k, time.Now(), "filechannels: ignoring a damaged mentions.json", err)
		return nil, nil
	}
	for i := range doc.Rows {
		doc.Rows[i].At = utc(doc.Rows[i].At)
	}
	return doc.Rows, nil
}

// writeRows replaces the mentions.json of channel k with rows, through the
// same atomic write meta.yaml takes.
func writeRows(d *disk, k channels.Key, rows []mentionRow) error {
	sortRows(rows)
	data, err := json.Marshal(mentionsDoc{V: mentionsVersion, Rows: rows})
	if err != nil { // coverage-ignore: invariant: plain data
		return fmt.Errorf("filechannels: encode addressed rows of %s: %w", k, err)
	}
	if err := d.rfs.WriteFile(mentionsKey(k), data, filePerm); err != nil {
		return fmt.Errorf("filechannels: write addressed rows of %s: %w", k, err)
	}
	return nil
}

func sortRows(rows []mentionRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Seq != b.Seq {
			return a.Seq < b.Seq
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.Kind < b.Kind
	})
}

// dropRows removes the rows of message seq that keep does not hold, and
// writes the file when that changed it. keep is called with each row of
// the message. The caller holds the lock and writes before the log record,
// so a failure fails the operation.
func dropRows(d *disk, k channels.Key, seq int64, keep func(mentionRow) bool) error {
	rows, err := readRows(d, chanDir(k), k)
	if err != nil {
		return err
	}
	out := rows[:0:0]
	for _, r := range rows {
		if r.Seq != seq || keep(r) {
			out = append(out, r)
		}
	}
	if len(out) == len(rows) {
		return nil
	}
	return writeRows(d, k, out)
}

// rebuildRows writes the rows of msg's mentions and its reply row that
// are missing, and returns whether a reply row is held afterwards. A
// mention row that exists keeps its time; a missing one is written at at.
// The reply row is kept when present, else written for replyTarget when
// that is set. New rows go through both caps (see [channels.Evict]). The
// caller holds the lock.
func rebuildRows(d *disk, k channels.Key, msg channels.Message, replyTarget string, at time.Time) (bool, error) {
	rows, err := readRows(d, chanDir(k), k)
	if err != nil {
		return false, err
	}
	held := map[[2]string]bool{}
	for _, r := range rows {
		if r.Seq == msg.Seq {
			held[[2]string{r.Target, string(r.Kind)}] = true
		}
	}
	var add []mentionRow
	newRow := func(target string, kind channels.AddressKind, t time.Time) mentionRow {
		return mentionRow{Seq: msg.Seq, Target: target, Kind: kind, At: t, Author: msg.Author, Type: msg.Type,
			ReplyTo: msg.ReplyTo}
	}
	for _, m := range msg.Mentions {
		if held[[2]string{m.Target, string(channels.AddressMention)}] {
			continue
		}
		add = append(add, newRow(m.Target, channels.AddressMention, at))
	}
	hasReply := false
	for key := range held {
		if key[1] == string(channels.AddressReply) {
			hasReply = true
		}
	}
	if !hasReply && replyTarget != "" {
		add = append(add, newRow(replyTarget, channels.AddressReply, msg.At))
	}
	if len(add) == 0 {
		return hasReply, nil
	}
	markers, err := readMarkers(d, chanDir(k), k)
	if err != nil {
		return false, err
	}
	markersOf := func(target string) []int64 {
		var out []int64
		for rd, r := range markers {
			if rd.User == target {
				out = append(out, r.Seq)
			}
		}
		return out
	}
	for _, nr := range add {
		var ok bool
		if rows, ok = makeRoom(rows, nr, markersOf); !ok {
			channels.LogSkippedRow(k, nr.Seq, nr.Target)
			continue
		}
		rows = append(rows, nr)
		hasReply = hasReply || nr.Kind == channels.AddressReply
	}
	return hasReply, writeRows(d, k, rows)
}

// makeRoom frees a slot for nr under the per-target cap and then under the
// per-file cap, and reports whether nr may be written. rows stays sorted
// by seq.
func makeRoom(rows []mentionRow, nr mentionRow, markers func(string) []int64) ([]mentionRow, bool) {
	sortRows(rows)
	var idx []int
	var mine []channels.AddressedRow
	for i, r := range rows {
		if r.Target == nr.Target {
			idx = append(idx, i)
			mine = append(mine, r.capRow())
		}
	}
	if len(mine) >= channels.MaxAddressedPerTarget {
		i, ok := channels.Evict(mine, markers, nr.Author.User)
		if !ok {
			return rows, false
		}
		rows = append(rows[:idx[i]], rows[idx[i]+1:]...)
	}
	if len(rows) >= channels.MaxAddressedPerFile {
		all := make([]channels.AddressedRow, len(rows))
		for i, r := range rows {
			all[i] = r.capRow()
		}
		i, ok := channels.Evict(all, markers, nr.Author.User)
		if !ok {
			return rows, false
		}
		rows = append(rows[:i], rows[i+1:]...)
	}
	return rows, true
}

// rowsAfterWrite writes the rows of msg after its log record is durable,
// as [rebuildRows] does. A failure is logged and swallowed, as
// markAfterWrite does: the message is stored, and only the notification is
// lost. It returns whether a reply row is held.
func rowsAfterWrite(d *disk, k channels.Key, msg channels.Message, replyTarget string, at time.Time) bool {
	replied, err := rebuildRows(d, k, msg, replyTarget, at)
	if err != nil {
		slog.Warn("filechannels: could not write the addressed rows of a message", "channel", k.String(),
			"seq", msg.Seq, "err", err)
		return false
	}
	return replied
}

// addressedCandidate is one row of the reader's target with what the
// marker filter needs.
type addressedCandidate struct {
	row    channels.Addressed
	marker int64
}

// ListAddressed implements [channels.Store]. It walks the tree once, as
// ListReads does: per channel directory it reads mentions.json, and for a
// directory with rows of the asked kind for the target the reads.json and
// meta.yaml. It keeps the target's newest [channels.MaxAddressedScan] rows
// of that kind before the marker filter, as the database scan does, then sorts and cuts in Go (see
// [channels.SortAddressed]).
func (s *Store) ListAddressed(
	_ context.Context, rd channels.Reader, kind channels.AddressKind, limit int,
) ([]channels.Addressed, error) {
	if err := channels.CheckListAddressed(rd, kind, limit); err != nil {
		return nil, err
	}
	var cands []addressedCandidate
	err := visitChannelDirs(&s.disk, func(k channels.Key, dir string) error {
		rows, err := readRows(&s.disk, dir, k)
		if err != nil {
			return err
		}
		var mine []mentionRow
		for _, r := range rows {
			if r.Target == rd.User && kind.Admits(r.Kind) {
				mine = append(mine, r)
			}
		}
		if len(mine) == 0 {
			return nil
		}
		m, err := readMetaAt(&s.disk, dir, k)
		switch {
		case errors.Is(err, channels.ErrNotFound):
			return nil
		case err != nil:
			s.damaged.Warn(k, time.Now(), "filechannels: skipping addressed rows whose meta.yaml cannot be read", err)
			return nil
		}
		markers, err := readMarkers(&s.disk, dir, k)
		if err != nil {
			return err
		}
		marker := markers[rd].Seq
		c := m.channel(k)
		for _, r := range mine {
			cands = append(cands, addressedCandidate{marker: marker, row: channels.Addressed{
				Channel: c, Seq: r.Seq, At: r.At, Kind: r.Kind, Author: r.Author, Type: r.Type, ReplyTo: r.ReplyTo,
			}})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i].row, cands[j].row
		if !a.At.Equal(b.At) {
			return a.At.After(b.At)
		}
		if a.Channel.Key != b.Channel.Key {
			return channels.KeyLess(b.Channel.Key, a.Channel.Key)
		}
		return a.Seq > b.Seq
	})
	if len(cands) > channels.MaxAddressedScan {
		cands = cands[:channels.MaxAddressedScan]
	}
	out := []channels.Addressed{}
	for _, c := range cands {
		if c.row.Seq > c.marker {
			out = append(out, c.row)
		}
	}
	channels.SortAddressed(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
