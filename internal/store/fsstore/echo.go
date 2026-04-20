package fsstore

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Sourcehaven-BV/rela/internal/cache"
)

// echoTracker isolates the "self-write vs external event" dedupe
// concern used by the filesystem watcher. The FS layer's post-write
// hook calls Recorded with the bytes that landed on disk; fsstore
// call sites that delete files call Forget; the watcher asks
// IsEcho when fsnotify delivers a change event.
//
// The data being hashed is whatever actually sits on disk — in a
// cleartext repo that's the raw bytes fsstore wrote; in an
// encrypted repo that's the sealed blob produced by cryptofs before
// SafeFS renamed it into place. The tracker doesn't care which;
// the hash comes from the bottom of the FS transform stack via
// SafeFS.OnPostWrite.
type echoTracker struct {
	hashes *cache.LRU[string, string]
}

func newEchoTracker(capacity int) *echoTracker {
	return &echoTracker{hashes: cache.NewLRU[string, string](capacity)}
}

// Recorded stores the hash of content that just landed at path.
// Signature matches storage.WriteObserver so it can be passed
// directly to SafeFS.OnPostWrite.
func (e *echoTracker) Recorded(path string, content []byte) {
	e.hashes.Put(path, hashContent(content))
}

// Forget drops any hash recorded for path. Called after the file
// is removed from disk so a future Put at the same path can't
// collide with a stale entry.
func (e *echoTracker) Forget(path string) {
	e.hashes.Delete(path)
}

// IsEcho reports whether data (typically bytes just read off disk
// after an fsnotify event) matches the most recent Recorded hash
// for path. When true, the watcher should treat the event as a
// self-write and skip reconciliation.
func (e *echoTracker) IsEcho(path string, data []byte) bool {
	cached, ok := e.hashes.Get(path)
	return ok && cached == hashContent(data)
}

// hashContent returns the hex-encoded SHA256 of content.
func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
