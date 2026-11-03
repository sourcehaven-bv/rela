package storage

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Temp files of atomic writes.
//
// SafeFS.WriteFile writes to a sibling temp file and renames it over the
// target; an fsstore attachment upload does the same with its own name
// ([AttachTempName]). Every sweep or skip of such leftovers (the fsstore
// startup sweep, `rela gc --temp-files`, the fsimport copy) uses the rules
// below, so the writers and their cleaners cannot drift apart.
//
// A temp name is "<base>.<16 hex>.tmp". The random part is unique per write,
// so several processes writing the same file never share a temp file. Two
// older forms are still recognized: "<base>.tmp" (the fixed SafeFS name
// before random names) and "<base>.new" (the legacy fsstore write).
const (
	tempSuffix       = ".tmp"
	legacyTempSuffix = ".new"

	// tempRandomBytes is the number of random bytes in a temp name,
	// written as twice as many hex characters.
	tempRandomBytes = 8

	// maxNameBytes is the file-name limit of common filesystems
	// (ext4, APFS, NTFS).
	maxNameBytes = 255
)

// AttachTempPrefix starts the name of an attachment upload in progress
// (see [AttachTempName]).
const AttachTempPrefix = ".rela-attach-tmp-"

// TempStaleAfter is how old a temp file must be before a sweep removes it.
// A write takes milliseconds even with fsync, so an older temp file is the
// leftover of a crash, not another process's write in flight.
const TempStaleAfter = 10 * time.Minute

// TempPath returns a new temp path for an atomic write of target, in the
// same directory. The base part is shortened when needed so the name fits
// the 255-byte limit; the temp name need not contain the full base.
func TempPath(target string) string {
	suffix := "." + randomHex() + tempSuffix
	dir, base := filepath.Split(target)
	return dir + truncateName(base, maxNameBytes-len(suffix)) + suffix
}

// AttachTempName returns a new name for an attachment upload in progress:
// [AttachTempPrefix] and a random part, so two processes uploading the same
// file name never share a temp file. The name has a fixed length, so it fits
// next to a stored name at the 255-byte limit.
func AttachTempName() string {
	return AttachTempPrefix + randomHex()
}

// randomHex returns tempRandomBytes random bytes as hex. crypto/rand.Read
// does not return an error since Go 1.24; it ends the program instead.
func randomHex() string {
	var b [tempRandomBytes]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// truncateName cuts name to at most n bytes without splitting a UTF-8
// character, because some filesystems (APFS) reject invalid UTF-8 names.
func truncateName(name string, n int) string {
	if len(name) <= n {
		return name
	}
	for n > 0 && !utf8.RuneStart(name[n]) {
		n--
	}
	return name[:n]
}

// IsTempName reports whether the file name (or path) is a temp file of an
// atomic write, in the current or an older form.
//
// It matches any name ending in ".tmp" or ".new", because the older forms
// carry no marker beyond the suffix and their leftovers must still be swept.
// That is only safe in trees whose file names rela chooses: entities/,
// relations/ and the state under .rela/, where every real file has a known
// suffix such as ".md". Never run it over user-named files, such as
// attachments, where "report.new" is a real upload; those trees use
// [IsAttachTempName].
func IsTempName(name string) bool {
	return strings.HasSuffix(name, tempSuffix) || strings.HasSuffix(name, legacyTempSuffix)
}

// IsAttachTempName reports whether the attachment file name is an upload in
// progress. A stored attachment name never starts with a dot
// (store.NormalizeFileName trims leading dots), so the prefix cannot match a
// real upload.
func IsAttachTempName(name string) bool {
	return strings.HasPrefix(name, AttachTempPrefix)
}

// IsStaleTemp reports whether a temp file last modified at modTime is old
// enough at now to be removed. A modTime in the future (clock skew) counts
// as fresh, so the file is removed by a later sweep instead. The mtime moves
// with every write, so a long upload that keeps writing stays fresh.
func IsStaleTemp(modTime, now time.Time) bool {
	return now.Sub(modTime) > TempStaleAfter
}
