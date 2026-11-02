package filechannels

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
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
// instead. A follower sees either as a gap.
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
)

// HintFeed is the hint journal of a [Store]. It is a realtime.HintSender
// and a realtime.HintSource.
type HintFeed struct {
	store *Store
	// rename renames a journal key; tests replace it to fail.
	rename func(oldKey, newKey string) error
	// poll is how often FollowHints looks; tests shorten it.
	poll time.Duration
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
	return &HintFeed{store: s, rename: s.rfs.Rename, poll: hintPoll}, nil
}

// SendHint appends payload as one line. The payload must not hold a
// newline. It rotates the journal first when it has grown past the
// rotation size.
func (f *HintFeed) SendHint(ctx context.Context, payload []byte) error {
	if bytes.IndexByte(payload, '\n') >= 0 || len(payload)+1 > hintMaxLine {
		return errors.New("filechannels: hint payload holds a newline or is too long")
	}
	if info, err := f.store.rfs.Stat(hintFile); err == nil && info.Size() > hintRotateBytes {
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
	if notExist(err) || (err == nil && info.Size() <= hintRotateBytes) {
		return nil // another writer rotated first
	}
	if err != nil {
		return fmt.Errorf("filechannels: stat hint journal: %w", err)
	}
	renameErr := f.rename(hintFile, hintOldFile)
	if renameErr == nil || info.Size() <= hintHardCapBytes {
		// A failed rename below the hard cap is retried by the next send.
		return nil
	}
	w, err := f.store.rfs.OpenForWrite(hintFile, hintFileMode)
	if err != nil {
		return fmt.Errorf("filechannels: truncate hint journal after %w: %w", renameErr, err)
	}
	return w.Close()
}

// FollowHints polls the journal until ctx ends. It starts at the journal's
// current end, so it reads only lines written after it started. It
// delivers each complete line, and reports a gap when the journal was
// replaced (another file, or a missing one after one was seen), shrank, or
// holds a line it cannot use. A partial last line waits for its newline.
func (f *HintFeed) FollowHints(ctx context.Context, deliver func([]byte), gap func()) error {
	t := &hintTail{feed: f, deliver: deliver, gap: gap}
	t.start()
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
}

// start takes the journal's current end as the position.
func (t *hintTail) start() {
	info, err := t.feed.store.rfs.Stat(hintFile)
	if err != nil {
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
		return // retried at the next poll
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
