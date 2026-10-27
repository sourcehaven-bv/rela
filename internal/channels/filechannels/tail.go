package filechannels

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// tailChunk is the first read from the end of a file. A message record is at
// most a few tens of kilobytes (the property cap plus its envelope), so one
// chunk usually holds the last complete line.
const tailChunk = 64 * 1024

// tail returns the last n bytes of the file at key, or all of it when it is
// shorter, and the file's size. A missing file returns no bytes and size 0.
//
// A filesystem whose files cannot be read at an offset (the in-memory one)
// reads the whole file; that costs memory, not disk reads.
func (d *disk) tail(key string, n int64) (data []byte, size int64, err error) {
	info, err := d.rfs.Stat(key)
	if notExist(err) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	size = info.Size()
	f, err := d.rfs.Open(key)
	if notExist(err) { // coverage-ignore: removed between Stat and Open; every writer holds the lock
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = f.Close() }()
	ra, ok := f.(io.ReaderAt)
	if !ok {
		all, readErr := io.ReadAll(f)
		if readErr != nil { // coverage-ignore: an in-memory read does not fail
			return nil, 0, readErr
		}
		return all[max(0, int64(len(all))-n):], int64(len(all)), nil
	}
	off := max(0, size-n)
	data = make([]byte, size-off)
	_, err = ra.ReadAt(data, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, err
	}
	return data, size, nil
}

// lastMessage returns the seq of the last complete message record in the
// file at key, and false when it holds none. Messages are written in seq
// order, and a rewrite keeps that order, so the last message record holds
// the highest seq. Patch records after it are skipped, and so is a torn
// final line. It reads from the end, doubling the read until it finds a
// message record or has read the whole file.
func (d *disk) lastMessage(key string) (seq int64, found bool, err error) {
	h, found, err := d.lastHead(key)
	return h.Seq, found, err
}

// recordHead is the part of a log record a tail scan reads: its kind, seq
// and time, and whether the message counts toward unread.
type recordHead struct {
	K           string            `json:"k"`
	Seq         int64             `json:"seq"`
	At          time.Time         `json:"at,omitzero"`
	TriggeredBy string            `json:"triggered_by,omitempty"`
	Removed     *channels.Removal `json:"removed,omitempty"`
}

// counts reports whether the message record h counts toward unread (see
// [channels.Counts]).
func (h recordHead) counts() bool {
	return channels.Counts(channels.Message{TriggeredBy: h.TriggeredBy, Removed: h.Removed})
}

// lastHead is lastMessage with the head of the record found.
func (d *disk) lastHead(key string) (recordHead, bool, error) {
	for n := int64(tailChunk); ; n *= 2 {
		data, size, readErr := d.tail(key, n)
		if readErr != nil {
			return recordHead{}, false, fmt.Errorf("filechannels: read %s: %w", key, readErr)
		}
		whole := int64(len(data)) == size
		if last, ok := lastMessageIn(tailBody(data, whole)); ok {
			return last, true, nil
		}
		if whole {
			return recordHead{}, false, nil
		}
	}
}

// tailBody is the complete lines of a tail read. The first line of a
// partial read may start mid-record, so it is dropped.
func tailBody(data []byte, whole bool) []byte {
	body := complete(data)
	if !whole {
		body = body[bytes.IndexByte(body, '\n')+1:]
	}
	return body
}

// lastMessageIn scans complete lines from the end for a message record.
// Damaged lines are skipped, as fold skips them.
func lastMessageIn(body []byte) (recordHead, bool) {
	var found recordHead
	ok := false
	eachFromEnd(body, func(h recordHead) bool {
		found, ok = h, true
		return false
	})
	return found, ok
}

// eachFromEnd calls fn with the head of each message record in body, the
// last one first, until fn returns false. Patch records and damaged lines
// are skipped.
func eachFromEnd(body []byte, fn func(recordHead) bool) {
	body = bytes.TrimSuffix(body, []byte{'\n'})
	for len(body) > 0 {
		i := bytes.LastIndexByte(body, '\n')
		raw := body[i+1:]
		body = body[:max(i, 0)]
		var head recordHead
		if json.Unmarshal(raw, &head) == nil && head.K == "m" && !fn(head) {
			return
		}
	}
}

// endsTorn reports whether the file at key ends in a torn line: it is not
// empty and its last byte is not a newline. Only the last byte is read.
func (d *disk) endsTorn(key string) (bool, error) {
	data, _, err := d.tail(key, 1)
	if err != nil {
		return false, err
	}
	return len(data) > 0 && data[len(data)-1] != '\n', nil
}
