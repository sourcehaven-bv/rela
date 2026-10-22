// Package filechannels is the [channels.Store] backend for the fs, memory
// and desktop tiers: one directory per channel under the project's .rela/
// directory.
//
// # Layout
//
//	<root>/<b32(id)>/<b32(face)|_>/<type>/<name|_>/meta.yaml
//	                                              /log-0000.jsonl ...
//	<root>/_audit/<b32(id)>.jsonl
//	<root>/_types/records.yaml
//	<root>/~detached/<b32(id)>-<stamp>/...
//
// b32 is lowercase unpadded base32. It keeps an entity id that differs
// only in case apart on a case-insensitive filesystem (APFS, NTFS), and its
// alphabet cannot spell a path separator, a dot or a Windows device name,
// so no id reaches the filesystem as anything but one plain segment. Type
// names follow the realtime.yaml grammar and channel names are minted, so
// both are safe as they are. `_` stands for "none" (no face, no name); no
// encoded segment can be `_`, and none starts with `_` or `~`, which mark
// the store's own directories. Every path goes through [storage.RootedFS].
//
// # Segments
//
// Messages live in JSON-lines segments of [SegmentSize] records; a message
// with seq n is in segment (n-1)/SegmentSize. A change to a message's
// metadata (reply count, action) is appended to that message's segment as a
// patch record and folded on read. A change to its content (edit, removal)
// rewrites the segment atomically through a temporary file, so the old
// content is gone from disk: the erasure the channels package promises.
//
// The last seq comes from the log itself, never from meta.yaml: the last
// complete message record of the newest segment, read from the end of the
// file. A crash mid-append leaves a torn final line; readers ignore it, and
// the next write to that segment cuts it off before appending. A segment
// that collects [SegmentSize] times two patch records is rewritten in
// folded form by the next patch.
//
// # Several processes, one directory
//
// The desktop app, `rela mcp`, CLI calls and rela-server can open one
// project at once. Every mutation, and [Store.Head], runs under a
// semaphore shared by every Store over the same directory in this process
// and then under the [Locker], a file lock that excludes other processes.
// Under the lock a mutation re-reads everything it needs from disk.
//
// meta.yaml is written before the log records of a mutation, so the rev a
// record carries is always reserved first: a crash in between leaves a gap
// in the revs, never a rev used twice. Head reads meta.yaml under the lock,
// so a poller never sees a rev whose records are not yet written.
//
// An append that moves the session (see [channels.SessionStep]) writes
// meta.yaml with the new session, then the message and the notice, and
// the session audit record last. A crash after meta.yaml can leave the
// session moved with no message, no notice and no audit record; a crash
// after the log records can leave the audit record missing. When the
// message ends a segment, the notice opens the next one in a separate
// write, so a crash between the two leaves the session moved and the
// message stored, with its [channels.Message.MovedSession] mark, but no
// notice and no audit record; the next append takes the notice's seq
// (TestSessionCrashBetweenSegments). The trail can miss a session record,
// but never names a wrong cause.
package filechannels

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// SegmentSize is the number of messages per log segment.
const SegmentSize = 1000

const (
	metaFile  = "meta.yaml"
	auditDir  = "_audit"
	typesFile = "_types/records.yaml"
	// legacyTypesFile is the type record of releases before cardinality
	// was kept: each type's anchor mode only. It is read, never written.
	legacyTypesFile = "_types/anchor-modes.yaml"
	detachedDir     = "~detached"
	trashDir        = "~trash"
	none            = "_"
	filePerm        = 0o600
)

// Locker excludes every other process from the store's directory for the
// span of one mutation. Lock blocks until the lock is held or ctx ends. An
// in-memory filesystem no other process can open takes
// filelock.ProcessPrivate.
type Locker interface {
	Lock(ctx context.Context) (unlock func(), err error)
}

// Store is the file backend. It adds locking and the [channels.Store]
// operations to the file layout that disk reads and writes.
//
// 20 -> 21 (TKT-ZU7P13): Untombstone. 21 -> 25 (TKT-964SIS):
// ConfirmTombstone, Tombstones, Purge and DropTypeRecord. The exported
// methods are the mandated [channels.Store] interface, so this is a
// required-interface exception rather than a ratchet target. Helpers stay
// free functions.
//
//plimsoll:max-exported-methods=27
type Store struct {
	disk
	locker Locker
	sem    chan struct{}
}

// disk reads and writes the file layout: heads, log segments, audit trails
// and the directory tree. It takes no lock; every caller holds the Store's.
type disk struct {
	rfs     *storage.RootedFS
	segSize int64
}

var _ channels.Store = (*Store)(nil)

// sems holds one semaphore per root directory, so two Stores over one
// directory in one process exclude each other even when their Locker is
// process-private.
var sems sync.Map

// New returns a store rooted at dir on fsys. The directory is created on
// the first write. A bare [storage.OsFS] is wrapped in a [storage.SafeFS];
// any other filesystem is used as given.
//
// Nil: fsys and locker rejected; a nil locker would hide the choice between
// a file lock and filelock.ProcessPrivate.
func New(fsys storage.FS, dir string, locker Locker) (*Store, error) {
	return newStore(fsys, dir, locker, SegmentSize)
}

func newStore(fsys storage.FS, dir string, locker Locker, segSize int64) (*Store, error) {
	if fsys == nil {
		return nil, errors.New("filechannels: filesystem must be non-nil")
	}
	if locker == nil {
		return nil, errors.New("filechannels: locker must be non-nil")
	}
	if osfs, ok := fsys.(*storage.OsFS); ok {
		// A rewrite must replace a segment atomically, or a crash leaves
		// it half written; SafeFS writes through a temporary file.
		fsys = storage.NewSafeFS(osfs)
	}
	rfs, err := storage.NewRootedFS(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("filechannels: %w", err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil { // coverage-ignore: NewRootedFS already resolved the same path
		return nil, fmt.Errorf("filechannels: %w", err)
	}
	v, _ := sems.LoadOrStore(filepath.Clean(abs), make(chan struct{}, 1))
	sem, _ := v.(chan struct{}) // sems holds only these
	return &Store{disk: disk{rfs: rfs, segSize: segSize}, locker: locker, sem: sem}, nil
}

// lock takes the shared semaphore and then the locker, and returns the
// release of both.
func (s *Store) lock(ctx context.Context) (func(), error) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("filechannels: lock: %w", ctx.Err())
	}
	unlock, err := s.locker.Lock(ctx)
	if err != nil {
		<-s.sem
		return nil, fmt.Errorf("filechannels: lock: %w", err)
	}
	return func() {
		unlock()
		<-s.sem
	}, nil
}

// --- paths ---

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func enc(s string) string { return strings.ToLower(b32.EncodeToString([]byte(s))) }

func dec(s string) (string, bool) {
	b, err := b32.DecodeString(strings.ToUpper(s))
	if err != nil || enc(string(b)) != s {
		return "", false
	}
	return string(b), true
}

func faceSeg(f entity.Face) string {
	if f == "" {
		return none
	}
	return enc(string(f))
}

func nameSeg(n string) string {
	if n == "" {
		return none
	}
	return n
}

func faceDir(a channels.Anchor) string { return enc(a.ID) + "/" + faceSeg(a.Face) }

func chanDir(k channels.Key) string {
	return faceDir(k.Anchor) + "/" + k.Type + "/" + nameSeg(k.Name)
}

func segKey(k channels.Key, idx int64) string {
	return fmt.Sprintf("%s/log-%04d.jsonl", chanDir(k), idx)
}

func auditKey(id string) string { return auditDir + "/" + enc(id) + ".jsonl" }

func notExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }

func (d *disk) exists(key string) (bool, error) {
	_, err := d.rfs.Stat(key)
	if notExist(err) {
		return false, nil
	}
	return err == nil, err
}

// readFile returns the file's bytes, or nil for a missing file.
func (d *disk) readFile(key string) ([]byte, error) {
	data, err := d.rfs.ReadFile(key)
	if notExist(err) {
		return nil, nil
	}
	return data, err
}

// readDir returns the names in a directory, or none for a missing one.
func (d *disk) readDir(key string) ([]string, error) {
	entries, err := d.rfs.ReadDir(key)
	if notExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// rootDirs returns the directories directly under the root. RootedFS
// accepts no key for the root itself, so this walks one level of it.
func (d *disk) rootDirs() ([]string, error) {
	var out []string
	err := d.rfs.WalkAll(func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if d.IsDir() {
			out = append(out, p)
			return fs.SkipDir
		}
		return nil
	})
	if notExist(err) {
		return nil, nil
	}
	return out, err
}

// --- meta ---

type pendingRef struct {
	Name string    `yaml:"name"`
	At   time.Time `yaml:"at"`
}

// meta is meta.yaml. SegRev holds the highest rev written to each segment,
// so a rev-cursor read skips segments with nothing newer.
type meta struct {
	Incarnation string               `yaml:"incarnation,omitempty"`
	State       channels.State       `yaml:"state"`
	Title       string               `yaml:"title,omitempty"`
	CreatedBy   channels.Author      `yaml:"created_by"`
	CreatedAt   time.Time            `yaml:"created_at"`
	ClosedAt    time.Time            `yaml:"closed_at,omitempty"`
	ArchivedAt  time.Time            `yaml:"archived_at,omitempty"`
	ReopenedAt  time.Time            `yaml:"reopened_at,omitempty"`
	LastSeq     int64                `yaml:"last_seq"`
	Rev         int64                `yaml:"rev"`
	SegRev      []int64              `yaml:"seg_rev,omitempty"`
	Pending     map[int64]pendingRef `yaml:"pending,omitempty"`
	// The session snapshot (see [channels.Session]). A meta.yaml from
	// before sessions has none and reads as unset.
	SessionState     string    `yaml:"session_state,omitempty"`
	SessionGen       int64     `yaml:"session_gen,omitempty"`
	SessionEnteredAt time.Time `yaml:"session_entered_at,omitempty"`
}

func (m *meta) setSession(s channels.Session) {
	m.SessionState, m.SessionGen, m.SessionEnteredAt = s.State, s.Generation, s.EnteredAt
}

func (m *meta) channel(k channels.Key) channels.Channel {
	return channels.Channel{
		Key: k, Incarnation: m.Incarnation, State: m.State, Title: m.Title, CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt.UTC(), ClosedAt: utc(m.ClosedAt), ArchivedAt: utc(m.ArchivedAt),
		ReopenedAt: utc(m.ReopenedAt),
		LastSeq:    m.LastSeq, Rev: m.Rev,
		Session: channels.Session{State: m.SessionState, Generation: m.SessionGen, EnteredAt: utc(m.SessionEnteredAt)},
	}
}

func utc(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return t.UTC()
}

// bump reserves the next rev for a record in segment idx.
func (m *meta) bump(idx int64) int64 {
	m.Rev++
	for int64(len(m.SegRev)) <= idx {
		m.SegRev = append(m.SegRev, 0)
	}
	m.SegRev[idx] = m.Rev
	return m.Rev
}

func (d *disk) readMeta(k channels.Key) (*meta, error) { return readMetaAt(d, chanDir(k), k) }

// readMetaAt reads the meta.yaml of channel k from the channel directory
// dir, which is chanDir(k) for a live channel.
func readMetaAt(d *disk, dir string, k channels.Key) (*meta, error) {
	data, err := d.readFile(dir + "/" + metaFile)
	if err != nil {
		return nil, fmt.Errorf("filechannels: read %s: %w", k, err)
	}
	if data == nil {
		return nil, fmt.Errorf("%w: channel %s", channels.ErrNotFound, k)
	}
	var m meta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("filechannels: channel %s has a damaged meta.yaml: %w", k, err)
	}
	return &m, nil
}

func (d *disk) writeMeta(k channels.Key, m *meta) error {
	data, err := yaml.Marshal(m)
	if err != nil { // coverage-ignore: invariant: meta is plain data
		return fmt.Errorf("filechannels: encode %s: %w", k, err)
	}
	if err := d.rfs.WriteFile(chanDir(k)+"/"+metaFile, data, filePerm); err != nil {
		return fmt.Errorf("filechannels: write %s: %w", k, err)
	}
	return nil
}

// --- records ---

// line is one record of a segment: a message ("m") or a patch ("p") to the
// message with the same seq in the same segment.
type line struct {
	K string `json:"k"`
	channels.Message
	ReplyCount *int             `json:"p_reply_count,omitempty"`
	Action     *channels.Action `json:"p_action,omitempty"`
	Clear      bool             `json:"p_clear,omitempty"`
}

func encodeLine(l line) []byte {
	b, err := json.Marshal(l)
	if err != nil { // coverage-ignore: invariant: props were encoded once already
		panic(fmt.Sprintf("filechannels: encode record: %v", err))
	}
	return append(b, '\n')
}

// complete returns data up to its last newline: a torn final line, left by
// a crash mid-append, is not part of the log.
func complete(data []byte) []byte {
	return data[:bytes.LastIndexByte(data, '\n')+1]
}

// fold folds a segment's records into its messages, ordered by seq.
func fold(data []byte, where string) []channels.Message {
	byseq := map[int64]*channels.Message{}
	var order []int64
	for raw := range bytes.SplitSeq(complete(data), []byte{'\n'}) {
		if len(raw) == 0 {
			continue
		}
		var l line
		if err := json.Unmarshal(raw, &l); err != nil {
			slog.Warn("filechannels: skipping a damaged record", "segment", where)
			continue
		}
		switch l.K {
		case "m":
			m := l.Message
			normalize(&m)
			if _, seen := byseq[m.Seq]; !seen {
				order = append(order, m.Seq)
			}
			byseq[m.Seq] = &m
		case "p":
			m, ok := byseq[l.Seq]
			if !ok {
				continue
			}
			m.Rev = l.Rev
			if l.ReplyCount != nil {
				m.ReplyCount = *l.ReplyCount
			}
			if l.Clear {
				m.Action = nil
			} else if l.Action != nil {
				a := *l.Action
				m.Action = &a
				normalizeAction(m.Action)
			}
		}
	}
	slices.Sort(order)
	out := make([]channels.Message, 0, len(order))
	for _, seq := range order {
		out = append(out, *byseq[seq])
	}
	return out
}

func normalize(m *channels.Message) {
	m.At = utc(m.At)
	m.EditedAt = utc(m.EditedAt)
	if m.Removed != nil {
		m.Removed.At = utc(m.Removed.At)
	}
	normalizeAction(m.Action)
}

func normalizeAction(a *channels.Action) {
	if a != nil {
		a.At = utc(a.At)
		a.PendingAt = utc(a.PendingAt)
	}
}

func find(msgs []channels.Message, seq int64) int {
	i := sort.Search(len(msgs), func(i int) bool { return msgs[i].Seq >= seq })
	if i < len(msgs) && msgs[i].Seq == seq {
		return i
	}
	return -1
}

// appendRecord appends rec to the file at key, whose current bytes are cur.
// A torn final line is cut off first, through an atomic rewrite.
func (d *disk) appendRecord(key string, cur, rec []byte) error {
	if len(cur) > 0 && cur[len(cur)-1] != '\n' {
		cur = complete(cur)
		if err := d.rfs.WriteFile(key, cur, filePerm); err != nil {
			return fmt.Errorf("filechannels: repair %s: %w", key, err)
		}
	}
	if !d.rfs.SupportsStreaming() {
		return d.rfs.WriteFile(key, append(cur, rec...), filePerm)
	}
	w, err := d.rfs.OpenForAppend(key, filePerm)
	if err != nil {
		return fmt.Errorf("filechannels: open %s: %w", key, err)
	}
	if _, err := w.Write(rec); err != nil { // coverage-ignore: os-fs-event: a full disk is not injectable here
		_ = w.Close()
		return fmt.Errorf("filechannels: append %s: %w", key, err)
	}
	if f, ok := w.(interface{ Sync() error }); ok {
		if err := f.Sync(); err != nil { // coverage-ignore: os-fs-event: fsync failure is not injectable here
			_ = w.Close()
			return fmt.Errorf("filechannels: sync %s: %w", key, err)
		}
	}
	return w.Close()
}

// rewrite replaces a segment with the compact form of msgs, atomically.
func (d *disk) rewrite(k channels.Key, idx int64, msgs []channels.Message) error {
	var buf bytes.Buffer
	for _, m := range msgs {
		buf.Write(encodeLine(line{K: "m", Message: m}))
	}
	if err := d.rfs.WriteFile(segKey(k, idx), buf.Bytes(), filePerm); err != nil {
		return fmt.Errorf("filechannels: rewrite %s: %w", segKey(k, idx), err)
	}
	return nil
}

// segments returns the channel's segment indices, ascending.
func (d *disk) segments(k channels.Key) ([]int64, error) {
	entries, err := d.rfs.ReadDir(chanDir(k))
	if notExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("filechannels: list %s: %w", k, err)
	}
	var out []int64
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "log-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(name, "log-"), ".jsonl"), 10, 64)
		if err == nil && n >= 0 {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out, nil
}

func (d *disk) segment(k channels.Key, idx int64) ([]byte, error) {
	data, err := d.readFile(segKey(k, idx))
	if err != nil {
		return nil, fmt.Errorf("filechannels: read %s: %w", segKey(k, idx), err)
	}
	return data, nil
}

// lastSeq reads the highest seq in the log, walking back over segments
// that hold no message. It reads each segment from its end (see
// disk.lastMessage), so its cost does not grow with the segment.
func (d *disk) lastSeq(k channels.Key) (int64, error) {
	idxs, err := d.segments(k)
	if err != nil {
		return 0, err
	}
	for i := len(idxs) - 1; i >= 0; i-- {
		seq, ok, err := d.lastMessage(segKey(k, idxs[i]))
		if err != nil {
			return 0, err
		}
		if ok {
			return seq, nil
		}
	}
	return 0, nil
}

func (d *disk) idx(seq int64) int64 { return (seq - 1) / d.segSize }

// loadMessage reads the segment holding seq and returns its folded
// messages and the position of seq, or ErrNotFound.
func (d *disk) loadMessage(k channels.Key, seq int64) ([]channels.Message, int, error) {
	if seq <= 0 {
		return nil, 0, fmt.Errorf("%w: message %d in %s", channels.ErrNotFound, seq, k)
	}
	data, err := d.segment(k, d.idx(seq))
	if err != nil {
		return nil, 0, err
	}
	msgs := fold(data, segKey(k, d.idx(seq)))
	i := find(msgs, seq)
	if i < 0 {
		return nil, 0, fmt.Errorf("%w: message %d in %s", channels.ErrNotFound, seq, k)
	}
	return msgs, i, nil
}

func (d *disk) appendAudit(id string, rec channels.AuditRecord) error {
	b, err := json.Marshal(rec)
	if err != nil { // coverage-ignore: invariant: plain data
		return err
	}
	return d.appendTo(auditKey(id), append(b, '\n'))
}

// appendTo appends rec to the file at key without reading it whole: only
// its last byte is read, to find a torn final line. A torn file, and a
// filesystem that cannot append in place, take the full read of
// appendRecord.
func (d *disk) appendTo(key string, rec []byte) error {
	torn := false
	if d.rfs.SupportsStreaming() {
		var err error
		torn, err = d.endsTorn(key)
		if err != nil {
			return fmt.Errorf("filechannels: read %s: %w", key, err)
		}
	}
	var cur []byte
	if torn || !d.rfs.SupportsStreaming() {
		var err error
		cur, err = d.readFile(key)
		if err != nil {
			return fmt.Errorf("filechannels: read %s: %w", key, err)
		}
	}
	return d.appendRecord(key, cur, rec)
}

// compactAfter returns the number of patch records a segment may hold
// before the next patch rewrites it compactly. Twice the segment size keeps
// a segment at most about three times its compact length.
func (d *disk) compactAfter() int { return int(2 * d.segSize) }

// appendLog appends rec, which may hold patch records, to segment idx of k,
// whose current bytes are cur. A segment that already holds compactAfter
// patch records is rewritten in folded form instead, so replies and action
// changes to old messages cannot grow it without bound.
func (d *disk) appendLog(k channels.Key, idx int64, cur, rec []byte) error {
	if patchCount(cur) < d.compactAfter() {
		return d.appendRecord(segKey(k, idx), cur, rec)
	}
	return d.rewrite(k, idx, fold(append(complete(cur), rec...), segKey(k, idx)))
}

// patchPrefix starts every patch line: encodeLine writes the kind first,
// and JSON escapes a newline inside a string, so the prefix cannot occur
// inside a record.
var patchPrefix = []byte(`{"k":"p"`)

func patchCount(data []byte) int {
	n := bytes.Count(data, append([]byte{'\n'}, patchPrefix...))
	if bytes.HasPrefix(data, patchPrefix) {
		n++
	}
	return n
}

// move renames from to to, creating the parent of to first.
func (d *disk) move(from, to string) error {
	if i := strings.LastIndexByte(to, '/'); i > 0 {
		if err := d.rfs.MkdirAll(to[:i], 0o755); err != nil {
			return err
		}
	}
	return d.rfs.Rename(from, to)
}

// removeAll removes a directory tree, files first.
func (d *disk) removeAll(key string) error {
	var files, dirs []string
	err := d.rfs.Walk(key, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
		return nil
	})
	if notExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := d.rfs.Remove(f); err != nil && !notExist(err) {
			return err
		}
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := d.rfs.Remove(dirs[i]); err != nil && !notExist(err) {
			return err
		}
	}
	return nil
}

// pruneEmpty removes the now-empty parent directories of a channel.
func (d *disk) pruneEmpty(k channels.Key) {
	for _, dir := range []string{faceDir(k.Anchor) + "/" + k.Type, faceDir(k.Anchor), enc(k.ID)} {
		names, err := d.rfs.ReadDir(dir)
		if notExist(err) {
			continue // already gone, or moved away by the caller
		}
		if err != nil || len(names) > 0 {
			return
		}
		if err := d.rfs.Remove(dir); err != nil {
			return
		}
	}
}

// --- listing ---

// visitFace calls fn for every channel at the face directory of anchor, in
// type and name order. typ narrows to one type.
func (d *disk) visitFace(a channels.Anchor, typ string, fn func(channels.Key, *meta) error) error {
	return visitFaceAt(d, faceDir(a), a, typ, fn)
}

// visitFaceAt calls fn for every channel in dir, laid out as the face
// directory of anchor a. A face batch under ~detached has that layout too.
func visitFaceAt(d *disk, dir string, a channels.Anchor, typ string, fn func(channels.Key, *meta) error) error {
	types, err := d.readDir(dir)
	if err != nil {
		return fmt.Errorf("filechannels: list %s: %w", a.ID, err)
	}
	sort.Strings(types)
	for _, t := range types {
		if (typ != "" && t != typ) || !channels.ValidName(t) {
			continue
		}
		names, err := d.readDir(dir + "/" + t)
		if err != nil {
			return fmt.Errorf("filechannels: list %s: %w", a.ID, err)
		}
		sort.Strings(names)
		for _, n := range names {
			k := channels.Key{Anchor: a, Type: t}
			if n != none {
				k.Name = n
			}
			if k.Validate() != nil {
				continue
			}
			m, err := readMetaAt(d, dir+"/"+t+"/"+n, k)
			if errors.Is(err, channels.ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if err := fn(k, m); err != nil {
				return err
			}
		}
	}
	return nil
}

// visitEntity calls fn for every channel of the entity id. typ narrows to
// one type.
func (d *disk) visitEntity(id, typ string, fn func(channels.Key, *meta) error) error {
	return visitEntityAt(d, enc(id), id, typ, fn)
}

// visitEntityAt calls fn for every channel in dir, laid out as the
// directory of entity id: one directory per face. A whole-entity batch
// under ~detached has that layout too.
func visitEntityAt(d *disk, dir, id, typ string, fn func(channels.Key, *meta) error) error {
	faces, err := d.readDir(dir)
	if err != nil {
		return fmt.Errorf("filechannels: list %s: %w", id, err)
	}
	var anchors []channels.Anchor
	for _, f := range faces {
		a := channels.Anchor{ID: id}
		if f != none {
			face, ok := dec(f)
			if !ok {
				continue
			}
			a.Face = entity.Face(face)
		}
		if a.Validate() == nil {
			anchors = append(anchors, a)
		}
	}
	sort.Slice(anchors, func(i, j int) bool { return anchors[i].Face < anchors[j].Face })
	for _, a := range anchors {
		if err := visitFaceAt(d, dir+"/"+faceSeg(a.Face), a, typ, fn); err != nil {
			return err
		}
	}
	return nil
}

// visitAll calls fn for every live channel. typ narrows to one type, and
// then only that type's metadata is read.
func (d *disk) visitAll(typ string, fn func(channels.Key, *meta) error) error {
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
		if err := d.visitEntity(id, typ, fn); err != nil {
			return err
		}
	}
	return nil
}
