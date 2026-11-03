package filechannels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// readsFile holds a channel's read markers, beside its meta.yaml. It moves
// with the channel directory on a rekey, tombstone, untombstone and purge,
// and goes with it on a delete.
const readsFile = "reads.json"

// readsVersion is the format of reads.json.
const readsVersion = 1

// readsDoc is reads.json: the markers sorted by user and organization.
type readsDoc struct {
	V       int         `json:"v"`
	Readers []readerRow `json:"readers"`
}

// readerRow is one marker in reads.json.
type readerRow struct {
	User string    `json:"user"`
	Org  string    `json:"org,omitempty"`
	Seq  int64     `json:"seq"`
	At   time.Time `json:"at"`
}

func readsKey(k channels.Key) string { return chanDir(k) + "/" + readsFile }

// readMarkers returns the markers of channel k, from the channel directory
// dir. A missing file holds none. A damaged file reads as none too, with a
// warning: the next moving mark replaces it.
func readMarkers(d *disk, dir string, k channels.Key) (map[channels.Reader]readerRow, error) {
	data, err := d.readFile(dir + "/" + readsFile)
	if err != nil {
		return nil, fmt.Errorf("filechannels: read markers of %s: %w", k, err)
	}
	out := map[channels.Reader]readerRow{}
	if data == nil {
		return out, nil
	}
	var doc readsDoc
	if err := json.Unmarshal(data, &doc); err != nil || doc.V != readsVersion {
		d.damaged.Warn(k, time.Now(), "filechannels: ignoring a damaged reads.json", err)
		return out, nil
	}
	for _, r := range doc.Readers {
		r.At = utc(r.At)
		out[channels.Reader{User: r.User, OrgID: r.Org}] = r
	}
	return out, nil
}

// writeMarkers replaces the reads.json of channel k with rs, through the
// same atomic write meta.yaml takes.
func writeMarkers(d *disk, k channels.Key, rs map[channels.Reader]readerRow) error {
	doc := readsDoc{V: readsVersion, Readers: make([]readerRow, 0, len(rs))}
	for _, r := range rs {
		doc.Readers = append(doc.Readers, r)
	}
	sort.Slice(doc.Readers, func(i, j int) bool {
		a, b := doc.Readers[i], doc.Readers[j]
		if a.User != b.User {
			return a.User < b.User
		}
		return a.Org < b.Org
	})
	data, err := json.Marshal(doc)
	if err != nil { // coverage-ignore: invariant: plain data
		return fmt.Errorf("filechannels: encode markers of %s: %w", k, err)
	}
	if err := d.rfs.WriteFile(readsKey(k), data, filePerm); err != nil {
		return fmt.Errorf("filechannels: write markers of %s: %w", k, err)
	}
	return nil
}

// raiseMarker sets rd's marker on k to seq when that raises it or creates
// it, at at, and writes reads.json. It reports whether the marker moved.
// The caller holds the lock.
func raiseMarker(d *disk, k channels.Key, rd channels.Reader, seq int64, at time.Time) (bool, error) {
	rs, err := readMarkers(d, chanDir(k), k)
	if err != nil {
		return false, err
	}
	old, ok := rs[rd]
	if ok && seq <= old.Seq {
		return false, nil
	}
	rs[rd] = readerRow{User: rd.User, Org: rd.OrgID, Seq: seq, At: channels.NormTime(at)}
	return true, writeMarkers(d, k, rs)
}

// markAfterWrite raises the author's marker after a write whose records
// are already durable. A failure is logged and swallowed: an error would
// make the client repeat a write that was stored. The worst case is the
// author's own message showing as unread to them.
func markAfterWrite(d *disk, k channels.Key, rd channels.Reader, seq int64, at time.Time) {
	if _, err := raiseMarker(d, k, rd, seq, at); err != nil {
		slog.Warn("filechannels: could not move the author's read marker", "channel", k.String(), "user", rd.User,
			"err", err)
	}
}

// unread counts the counting messages of k above seq, up to lastSeq, at
// most [channels.UnreadCap]. It reads each segment from its end, doubling
// the read as lastMessage does, and stops at a message at or below seq or
// at the cap, so its cost follows the unread messages, not the segment.
// Patch records change only reply counts and actions, never whether a
// message counts, so the scan skips them.
func unread(d *disk, k channels.Key, seq, lastSeq int64) (int, error) {
	if lastSeq <= seq {
		return 0, nil
	}
	total := 0
	for idx := d.idx(lastSeq); idx >= d.idx(seq+1) && idx >= 0; idx-- {
		n, done, err := unreadIn(d, segKey(k, idx), seq, channels.UnreadCap-total)
		if err != nil {
			return 0, err
		}
		total += n
		if done || total >= channels.UnreadCap {
			break
		}
	}
	return min(total, channels.UnreadCap), nil
}

// unreadIn counts the counting messages above seq in the segment at key,
// at most limit. done reports that the scan reached a message at or below
// seq or the limit, so older segments need no read.
func unreadIn(d *disk, key string, seq int64, limit int) (n int, done bool, err error) {
	for size := int64(tailChunk); ; size *= 2 {
		data, total, readErr := d.tail(key, size)
		if readErr != nil {
			return 0, false, fmt.Errorf("filechannels: read %s: %w", key, readErr)
		}
		whole := int64(len(data)) == total
		n, done = 0, false
		eachFromEnd(tailBody(data, whole), func(h recordHead) bool {
			if h.Seq <= seq {
				done = true
				return false
			}
			if h.counts() {
				n++
			}
			if n >= limit {
				done = true
				return false
			}
			return true
		})
		if done || whole {
			return n, done, nil
		}
	}
}

// logHead returns the last seq of k's log and the time of that message,
// walking back over segments that hold no message, as lastSeq does.
func (d *disk) logHead(k channels.Key) (int64, time.Time, error) {
	idxs, err := d.segments(k)
	if err != nil {
		return 0, time.Time{}, err
	}
	for i := len(idxs) - 1; i >= 0; i-- {
		h, ok, err := d.lastHead(segKey(k, idxs[i]))
		if err != nil {
			return 0, time.Time{}, err
		}
		if ok {
			return h.Seq, utc(h.At), nil
		}
	}
	return 0, time.Time{}, nil
}

// readState is the marker r of channel k with what lies above it.
func readState(d *disk, k channels.Key, r readerRow, lastSeq int64) (channels.ReadState, error) {
	n, err := unread(d, k, r.Seq, lastSeq)
	if err != nil {
		return channels.ReadState{}, err
	}
	return channels.ReadState{Seq: r.Seq, At: r.At, LastSeq: lastSeq, Unread: n}, nil
}

// SetRead implements [channels.Store]. A seq that is not above the
// marker, read without the lock, is answered at once: the marker only
// rises, so a stale read costs at most one redundant lock. A moving mark
// takes the lock, re-reads the last seq from the log, and writes
// reads.json; it writes no meta.yaml and no rev. The lock is the
// project-wide one every mutation takes.
func (s *Store) SetRead(
	ctx context.Context, k channels.Key, rd channels.Reader, seq int64, at time.Time,
) (channels.ReadState, bool, error) {
	if err := channels.CheckSetRead(k, rd, seq); err != nil {
		return channels.ReadState{}, false, err
	}
	if st, ok, err := setReadNoop(s, k, rd, seq); err != nil || ok {
		return st, false, err
	}
	unlock, err := s.lock(ctx)
	if err != nil {
		return channels.ReadState{}, false, err
	}
	defer unlock()
	if _, err = s.readMeta(k); err != nil {
		return channels.ReadState{}, false, err
	}
	last, err := s.lastSeq(k)
	if err != nil {
		return channels.ReadState{}, false, err
	}
	rs, err := readMarkers(&s.disk, chanDir(k), k)
	if err != nil {
		return channels.ReadState{}, false, err
	}
	old, had := rs[rd]
	target := channels.ClampRead(old.Seq, seq, last)
	moved := !had || target > old.Seq
	if moved {
		old = readerRow{User: rd.User, Org: rd.OrgID, Seq: target, At: channels.NormTime(at)}
		rs[rd] = old
		err = writeMarkers(&s.disk, k, rs)
		if err != nil {
			return channels.ReadState{}, false, err
		}
	}
	st, err := readState(&s.disk, k, old, last)
	return st, moved, err
}

// setReadNoop answers a SetRead that would not move the marker, without
// the lock. ok is false when the marker is missing or would rise.
func setReadNoop(s *Store, k channels.Key, rd channels.Reader, seq int64) (channels.ReadState, bool, error) {
	if _, err := s.readMeta(k); err != nil {
		return channels.ReadState{}, false, err
	}
	rs, err := readMarkers(&s.disk, chanDir(k), k)
	if err != nil {
		return channels.ReadState{}, false, err
	}
	old, had := rs[rd]
	if !had {
		return channels.ReadState{}, false, nil
	}
	last, err := s.lastSeq(k)
	if err != nil {
		return channels.ReadState{}, false, err
	}
	if channels.ClampRead(old.Seq, seq, last) > old.Seq {
		return channels.ReadState{}, false, nil
	}
	st, err := readState(&s.disk, k, old, last)
	return st, err == nil, err
}

// Reads implements [channels.Store]. It takes no lock, as ListSessionsDue:
// per key it reads reads.json, then the log tail, and counts only when
// the last seq is above the marker.
func (s *Store) Reads(
	_ context.Context, rd channels.Reader, keys []channels.Key,
) (map[channels.Key]channels.ReadState, error) {
	if err := channels.CheckReads(rd, keys); err != nil {
		return nil, err
	}
	out := make(map[channels.Key]channels.ReadState, len(keys))
	for _, k := range keys {
		rs, err := readMarkers(&s.disk, chanDir(k), k)
		if err != nil {
			return nil, err
		}
		r, ok := rs[rd]
		if !ok {
			continue
		}
		last, err := s.lastSeq(k)
		if err != nil {
			return nil, err
		}
		st, err := readState(&s.disk, k, r, last)
		if err != nil {
			return nil, err
		}
		out[k] = st
	}
	return out, nil
}

// ListReads implements [channels.Store]. It walks the tree once, as
// ListSessionsDue does but over every type: per channel directory it reads
// reads.json, and for a directory that holds the reader the meta.yaml and
// the log tail. It sorts and cuts in Go (see [channels.SortReadWindow]).
// The ~detached and ~trash trees are outside the walk.
func (s *Store) ListReads(_ context.Context, rd channels.Reader, limit int) ([]channels.ReadMarker, error) {
	if err := channels.CheckListReads(rd, limit); err != nil {
		return nil, err
	}
	out := []channels.ReadMarker{}
	err := visitChannelDirs(&s.disk, func(k channels.Key, dir string) error {
		rs, err := readMarkers(&s.disk, dir, k)
		if err != nil {
			return err
		}
		r, ok := rs[rd]
		if !ok {
			return nil
		}
		m, err := readMetaAt(&s.disk, dir, k)
		switch {
		case errors.Is(err, channels.ErrNotFound):
			return nil
		case err != nil:
			s.damaged.Warn(k, time.Now(), "filechannels: skipping a read marker whose meta.yaml cannot be read", err)
			return nil
		}
		last, logAt, err := s.logHead(k)
		if err != nil {
			return err
		}
		c := m.channel(k)
		c.LastSeq = last
		msgAt := m.LastAt
		if msgAt.IsZero() {
			msgAt = logAt
		}
		out = append(out, channels.ReadMarker{
			Channel: c, Seq: r.Seq, At: r.At, LastAt: channels.LastActivity(c, msgAt),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return channels.SortReadWindow(out, limit), nil
}

// visitChannelDirs calls fn with the key and directory of every live
// channel directory, of every type, without reading anything in it.
func visitChannelDirs(d *disk, fn func(channels.Key, string) error) error {
	ids, err := d.rootDirs()
	if err != nil {
		return fmt.Errorf("filechannels: list: %w", err)
	}
	for _, name := range ids {
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, "~") {
			continue
		}
		id, ok := dec(name)
		if !ok || entity.ValidateID(id) != nil {
			continue
		}
		faces, err := d.readDir(name)
		if err != nil {
			return fmt.Errorf("filechannels: list %s: %w", id, err)
		}
		for _, f := range faces {
			a := channels.Anchor{ID: id}
			if f != none {
				face, ok := dec(f)
				if !ok {
					continue
				}
				a.Face = entity.Face(face)
			}
			if a.Validate() != nil {
				continue
			}
			if err := visitFaceDirs(d, name+"/"+f, a, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// visitFaceDirs is visitChannelDirs for one face directory dir of a.
func visitFaceDirs(d *disk, dir string, a channels.Anchor, fn func(channels.Key, string) error) error {
	typeDirs, err := d.readDir(dir)
	if err != nil {
		return fmt.Errorf("filechannels: list %s: %w", a.ID, err)
	}
	for _, t := range typeDirs {
		if !channels.ValidName(t) {
			continue
		}
		names, err := d.readDir(dir + "/" + t)
		if err != nil {
			return fmt.Errorf("filechannels: list %s: %w", a.ID, err)
		}
		for _, n := range names {
			k := channels.Key{Anchor: a, Type: t}
			if n != none {
				k.Name = n
			}
			if k.Validate() != nil {
				continue
			}
			if err := fn(k, dir+"/"+t+"/"+n); err != nil {
				return err
			}
		}
	}
	return nil
}
