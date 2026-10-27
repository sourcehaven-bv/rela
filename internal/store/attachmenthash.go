package store

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
)

// HashAttachment returns the lowercase hex SHA-256 of data, the form
// [AttachmentInfo.SHA256] carries (TKT-1TS55R).
func HashAttachment(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// AttachmentHasher hashes attachment bytes as they stream past, for a
// backend that does not hold the whole file in memory.
type AttachmentHasher struct{ h hash.Hash }

// NewAttachmentHasher returns an empty hasher.
func NewAttachmentHasher() *AttachmentHasher {
	return &AttachmentHasher{h: sha256.New()}
}

// Write adds p to the hash. It never fails.
func (a *AttachmentHasher) Write(p []byte) (int, error) { return a.h.Write(p) }

// Sum returns the lowercase hex SHA-256 of everything written so far.
func (a *AttachmentHasher) Sum() string { return hex.EncodeToString(a.h.Sum(nil)) }
