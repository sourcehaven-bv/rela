// Package attachment provides content-addressable storage for file attachments.
package attachment

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"path/filepath"
	"strings"
)

// HashReader computes the SHA-256 hash of data from a reader.
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashBytes computes the SHA-256 hash of a byte slice.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// PathFromHash constructs the relative path for an attachment given its hash and extension.
// Returns path like "attachments/ab/ab3f8c2e9d1a5b6c.png".
func PathFromHash(hash, ext string) string {
	prefix := hash[:2]
	filename := hash + ext
	return filepath.Join(AttachmentsDir, prefix, filename)
}

// ParsePath extracts hash and extension from an attachment path.
// Path format: "attachments/ab/ab3f8c2e9d1a5b6c.png"
// Returns hash "ab3f8c2e9d1a5b6c" and extension ".png".
func ParsePath(path string) (hash, ext string, ok bool) {
	// Normalize path separators
	path = filepath.ToSlash(path)

	// Must start with attachments/
	if !strings.HasPrefix(path, AttachmentsDir+"/") {
		return "", "", false
	}

	// Must have at least 3 components: attachments/prefix/hash.ext
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return "", "", false
	}

	// Get filename (last component)
	filename := filepath.Base(path)

	// Must have an extension
	ext = filepath.Ext(filename)
	if ext == "" {
		return "", "", false
	}

	hash = strings.TrimSuffix(filename, ext)

	// Validate hash format (must be hex and at least 8 chars)
	if len(hash) < 8 || !isHex(hash) {
		return "", "", false
	}

	return hash, ext, true
}

// MetadataPath returns the path to the metadata sidecar for an attachment path.
// For "attachments/ab/ab3f8c2e.png" returns "attachments/ab/ab3f8c2e.png.yaml".
func MetadataPath(attachmentPath string) string {
	return attachmentPath + ".yaml"
}

// isHex checks if a string contains only hexadecimal characters.
func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
