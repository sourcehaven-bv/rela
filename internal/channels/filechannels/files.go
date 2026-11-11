package filechannels

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// Files of a message live beside its log, one file per id:
//
//	<channel>/files/<seq>/<id>
//
// An append writes the bytes to <root>/~staging/<name>/ before it takes
// the lock, so a large file is never written while every other writer
// waits. Under the lock it renames the staged directory to files/<seq>,
// after removing any directory a crashed append left at that seq. When a
// write after that rename fails before the log record, the append moves
// files/<seq> to the trash, unless a stashed job move may still replay
// the record. A crash in that window leaves files under a seq the log
// does not hold; the next append at that seq removes them, with or
// without files of its own.
//
// A removal moves files/<seq> to the trash before it rewrites the
// segment, so a crash in between leaves a message that lists files whose
// bytes are gone: a read fails with not found rather than serving bytes
// the removal was to erase. Delete, tombstone and purge move the whole
// channel directory, files included. Every writer empties the trash under
// the lock.
const (
	filesDir   = "files"
	stagingDir = "~staging"
	// stagingTTL is the age after which a staged directory counts as
	// abandoned by a crashed append. It is far above the time one append
	// takes, so a sweep never takes a directory another append still
	// uses.
	stagingTTL = time.Hour
)

func filesKey(k channels.Key, seq int64) string {
	return chanDir(k) + "/" + filesDir + "/" + strconv.FormatInt(seq, 10)
}

// stageFiles writes the bytes of news under the ids of files into a new
// staging directory and returns its key. The caller holds no lock.
func stageFiles(d *disk, files []channels.File, news []channels.NewFile) (string, error) {
	dir := stagingDir + "/" + channels.NewName()
	for i, f := range files {
		if err := d.rfs.WriteFile(dir+"/"+f.ID, news[i].Data, filePerm); err != nil {
			_ = d.removeAll(dir)
			return "", fmt.Errorf("filechannels: stage file: %w", err)
		}
	}
	return dir, nil
}

// placeFiles moves the staged directory to the files of message seq in k,
// replacing what a crashed append left there. The caller holds the lock.
func placeFiles(d *disk, k channels.Key, seq int64, staged string) error {
	key := filesKey(k, seq)
	if err := d.removeAll(key); err != nil {
		return fmt.Errorf("filechannels: clear files of %d in %s: %w", seq, k, err)
	}
	if err := d.move(staged, key); err != nil {
		return fmt.Errorf("filechannels: place files of %d in %s: %w", seq, k, err)
	}
	return nil
}

// sweepStaging removes staged directories older than [stagingTTL]. A
// failure only leaves them for the next sweep.
func sweepStaging(d *disk, now time.Time) {
	entries, err := d.rfs.ReadDir(stagingDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < stagingTTL {
			continue
		}
		if err := d.removeAll(stagingDir + "/" + e.Name()); err != nil {
			slog.Warn("filechannels: sweep staged files", "dir", e.Name(), "error", err)
		}
	}
}

// trashFiles moves the files of message seq in k to the trash. The caller
// holds the lock and empties the trash after its write.
func trashFiles(d *disk, k channels.Key, seq int64) error {
	key := filesKey(k, seq)
	ok, err := d.exists(key)
	if err != nil || !ok {
		return err
	}
	return d.move(key, trashDir+"/"+channels.NewName())
}

// clearFiles removes the files a crashed or failed append left at seq in
// k, which the log does not hold. The caller holds the lock.
func clearFiles(d *disk, k channels.Key, seq int64) error {
	if err := d.removeAll(filesKey(k, seq)); err != nil {
		return fmt.Errorf("filechannels: clear files of %d in %s: %w", seq, k, err)
	}
	return nil
}

// settleFiles puts the files of the append at seq in place: the staged
// directory, or with none staged, a clear of what a crashed or failed
// append left at seq. The caller holds the lock, and calls discard under
// it when the log write fails.
func settleFiles(d *disk, k channels.Key, m *meta, seq int64, staged string) (discard func(), err error) {
	if staged == "" {
		return func() {}, clearFiles(d, k, seq)
	}
	sweepStaging(d, time.Now())
	if err := placeFiles(d, k, seq, staged); err != nil {
		return nil, err
	}
	return func() { discardFiles(d, k, m, seq) }, nil
}

// discardFiles moves the files placed for message seq in k to the trash
// and empties it, after an append failed. It keeps them when the message
// may still reach the log: when its record is there, or when m stashes it
// for a replay. A failure only leaves the files for the next append at
// seq. The caller holds the lock.
func discardFiles(d *disk, k channels.Key, m *meta, seq int64) {
	for _, r := range m.JobRecords {
		if r.Seq == seq {
			return
		}
	}
	last, err := d.lastSeq(k)
	if err != nil || last >= seq {
		return
	}
	if err := trashFiles(d, k, seq); err != nil {
		slog.Warn("filechannels: discard files of failed append", "channel", k.String(), "seq", seq, "error", err)
		return
	}
	emptyTrash(d, k)
}

// emptyTrash removes the trash. A failure only leaves it for the next
// delete or removal: what it holds is out of reach already. The caller
// holds the lock.
func emptyTrash(d *disk, k channels.Key) {
	if err := d.removeAll(trashDir); err != nil {
		slog.Warn("filechannels: empty trash", "channel", k.String(), "error", err)
	}
}

// OpenFile implements [channels.Store]. It takes no lock, like
// [Store.Get]: a removal that lands between the two reads makes the open
// fail with not found.
func (s *Store) OpenFile(
	ctx context.Context, k channels.Key, seq int64, id string,
) (channels.File, io.ReadCloser, error) {
	m, err := s.Get(ctx, k, seq)
	if err != nil {
		return channels.File{}, nil, err
	}
	f, ok := channels.FileOf(m, id)
	if !ok || !channels.ValidFileID(id) {
		return channels.File{}, nil, fmt.Errorf("%w: file %s of message %d in %s", channels.ErrNotFound, id, seq, k)
	}
	rc, err := s.rfs.Open(filesKey(k, seq) + "/" + id)
	if notExist(err) {
		return channels.File{}, nil, fmt.Errorf("%w: file %s of message %d in %s", channels.ErrNotFound, id, seq, k)
	}
	if err != nil {
		return channels.File{}, nil, fmt.Errorf("filechannels: open file %s of message %d in %s: %w", id, seq, k, err)
	}
	return f, rc, nil
}
