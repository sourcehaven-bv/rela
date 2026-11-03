package filechannels

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"sync/atomic"
	"time"
)

// The hint journal carries channel hints between the processes that share a
// channel directory (see realtime.HintSender). It is one append-only file of
// JSON lines in the _feed directory, which the store's walks skip. Each
// follower polls its size and reads the lines added since its last poll.
//
// The journal is rotated under the store's lock once it passes
// hintRotateBytes: it is renamed to hintOldFile, and the next append starts
// a new file. Where the rename fails (Windows, with a reader holding the
// file open) and the journal passes hintHardCapBytes, it is truncated
// instead. A rename that failed is tried again only once the journal grew
// by another hintRotateBytes, so a held file does not cost the store's lock
// on every send. A follower sees a rotation or a truncation as a gap.

// hintFileMode keeps the journal readable by its owner only.
const hintFileMode = 0o600

const (
	hintFile         = "_feed/changes.jsonl"
	hintOldFile      = "_feed/changes.old.jsonl"
	hintRotateBytes  = 1 << 20
	hintHardCapBytes = 4 << 20
	// hintMaxLine bounds one line; a longer one is unusable.
	hintMaxLine = 8 << 10
	// hintPoll is how often a follower looks at the journal.
	hintPoll = 250 * time.Millisecond
	// hintErrorLogGap is the shortest time between two logged read errors.
	hintErrorLogGap = time.Minute
)

// HintFeed is the hint journal of a [Store]. It is a realtime.HintSender
// and a realtime.HintSource.
type HintFeed struct {
	store *Store
	// rename renames a journal key; tests replace it to fail.
	rename func(oldKey, newKey string) error
	// poll is how often FollowHints looks; tests shorten it.
	poll time.Duration
	// rotateAt is the size past which a send rotates; tests lower it.
	rotateAt int64
	// retryAt is the size from which a send tries again to rotate after a
	// rename failed; zero when none failed.
	retryAt atomic.Int64
}

// NewHintFeed returns the hint journal of s. A store on a filesystem that
// cannot append (the in-memory one) has no other process to tell, and gets
// an error.
//
// Nil: s is rejected.
func NewHintFeed(s *Store) (*HintFeed, error) {
	if s == nil {
		return nil, errors.New("filechannels: hint feed needs a store")
	}
	if !s.rfs.SupportsStreaming() {
		return nil, errors.New("filechannels: hint feed needs a filesystem that can append")
	}
	return &HintFeed{store: s, rename: s.rfs.Rename, poll: hintPoll, rotateAt: hintRotateBytes}, nil
}

// SendHint appends payload as one line. The payload must not hold a
// newline. It rotates the journal first when it has grown past the
// rotation size.
func (f *HintFeed) SendHint(ctx context.Context, payload []byte) error {
	if bytes.IndexByte(payload, '\n') >= 0 || len(payload)+1 > hintMaxLine {
		return errors.New("filechannels: hint payload holds a newline or is too long")
	}
	if info, err := f.store.rfs.Stat(hintFile); err == nil && f.dueForRotation(info.Size()) {
		if err := f.rotate(ctx); err != nil {
			return err
		}
	}
	w, err := f.store.rfs.OpenForAppend(hintFile, hintFileMode)
	if err != nil {
		return fmt.Errorf("filechannels: open hint journal: %w", err)
	}
	// One write, so concurrent appenders never interleave inside a line.
	_, err = w.Write(append(bytes.Clone(payload), '\n'))
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("filechannels: append hint: %w", err)
	}
	return nil
}

// dueForRotation reports whether a journal of size should be rotated now.
func (f *HintFeed) dueForRotation(size int64) bool {
	return size > f.rotateAt && size >= f.retryAt.Load()
}

// rotate renames the journal away under the store's lock, so two writers
// never both rotate. When the rename fails and the journal is past the hard
// cap, it is truncated instead.
func (f *HintFeed) rotate(ctx context.Context) error {
	unlock, err := f.store.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	info, err := f.store.rfs.Stat(hintFile)
	if notExist(err) || (err == nil && info.Size() <= f.rotateAt) {
		return nil // another writer rotated first
	}
	if err != nil {
		return fmt.Errorf("filechannels: stat hint journal: %w", err)
	}
	renameErr := f.rename(hintFile, hintOldFile)
	if renameErr == nil {
		f.retryAt.Store(0)
		return nil
	}
	if info.Size() <= hintHardCapBytes {
		// Tried again once the journal grew, or at the hard cap.
		f.retryAt.Store(min(info.Size()+f.rotateAt, hintHardCapBytes+1))
		return nil
	}
	f.retryAt.Store(0)
	w, err := f.store.rfs.OpenForWrite(hintFile, hintFileMode)
	if err != nil {
		return fmt.Errorf("filechannels: truncate hint journal after %w: %w", renameErr, err)
	}
	return w.Close()
}

// FollowHints polls the journal until ctx ends. It starts at the journal's
// current end, so it reads only lines written after it started, and calls
// gap once it took that position (see realtime.HintSource). It delivers each complete line, and reports a gap when the journal was
// replaced (another file, or a missing one after one was seen), shrank, or
// holds a line it cannot use. A partial last line waits for its newline.
func (f *HintFeed) FollowHints(ctx context.Context, deliver func([]byte), gap func()) error {
	t := &hintTail{feed: f, deliver: deliver, gap: gap}
	t.start()
	gap()
	ticker := time.NewTicker(f.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			t.poll()
		}
	}
}

// hintTail is one follower's position in the journal.
type hintTail struct {
	feed    *HintFeed
	deliver func([]byte)
	gap     func()
	// info is the journal followed, nil when none was seen yet.
	info os.FileInfo
	off  int64
	// partial is a last line still waiting for its newline.
	partial []byte
	// skip drops the rest of an oversized line, up to its newline.
	skip bool
	// lastErrLog is when a read error was last logged.
	lastErrLog time.Time
}

// failed logs a read error, at most once per hintErrorLogGap. The poll
// retries; a journal this process cannot read (another user's, with mode
// 0600) keeps failing, and the log says so.
func (t *hintTail) failed(op string, err error) {
	now := time.Now()
	if !t.lastErrLog.IsZero() && now.Sub(t.lastErrLog) < hintErrorLogGap {
		return
	}
	t.lastErrLog = now
	slog.Warn("channels: cannot read the channel hint journal; other processes' changes show at the next refetch",
		"op", op, "error", err)
}

// start takes the journal's current end as the position.
func (t *hintTail) start() {
	info, err := t.feed.store.rfs.Stat(hintFile)
	if err != nil {
		if !notExist(err) {
			t.failed("stat", err)
		}
		return
	}
	t.info, t.off = info, info.Size()
}

// poll reads what was appended since the last poll.
func (t *hintTail) poll() {
	info, err := t.feed.store.rfs.Stat(hintFile)
	switch {
	case notExist(err):
		if t.info != nil {
			// Rotated away; the next file is new and read from its start.
			t.reset(nil)
		}
		return
	case err != nil:
		t.failed("stat", err) // retried at the next poll
		return
	}
	if t.info != nil && (!os.SameFile(t.info, info) || info.Size() < t.off) {
		t.reset(info)
	} else if t.info == nil {
		t.info = info
	}
	if info.Size() == t.off {
		return
	}
	t.read(info.Size())
}

// reset reports a gap and starts over at the start of info's file.
func (t *hintTail) reset(info os.FileInfo) {
	t.gap()
	t.info, t.off, t.partial, t.skip = info, 0, nil, false
}

// read reads the bytes from the position to size and delivers the
// complete lines among them.
func (t *hintTail) read(size int64) {
	if size-t.off > hintHardCapBytes {
		// More than a journal holds between rotations: skip to the end.
		t.gap()
		t.off, t.partial, t.skip = size, nil, true
		return
	}
	r, err := t.feed.store.rfs.Open(hintFile)
	if err != nil {
		t.failed("open", err)
		return
	}
	defer func() { _ = r.Close() }()
	ra, ok := r.(io.ReaderAt)
	if !ok { // coverage-ignore: NewHintFeed admits only filesystems whose files read at an offset
		return
	}
	buf := make([]byte, size-t.off)
	n, err := ra.ReadAt(buf, t.off)
	if err != nil && !errors.Is(err, io.EOF) {
		t.failed("read", err)
		return
	}
	t.off += int64(n)
	data := slices.Concat(t.partial, buf[:n])
	t.partial = nil
	for {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			break
		}
		line := data[:i]
		data = data[i+1:]
		if t.skip {
			t.skip = false
			continue
		}
		if len(line) == 0 || len(line) > hintMaxLine {
			t.gap()
			continue
		}
		t.deliver(line)
	}
	if t.skip {
		return // still inside an oversized line
	}
	if len(data) > hintMaxLine {
		t.gap()
		t.skip = true
		return
	}
	t.partial = bytes.Clone(data)
}
