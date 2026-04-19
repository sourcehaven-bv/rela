package encryption

import (
	"crypto/rand"
	"fmt"
	"io"
)

const DataKeySize = 32

// NewDataKey returns a cryptographically random 32-byte AES-256 data key.
func NewDataKey() ([]byte, error) {
	return newDataKey(rand.Reader)
}

func newDataKey(r io.Reader) ([]byte, error) {
	k := make([]byte, DataKeySize)
	if _, err := io.ReadFull(r, k); err != nil {
		return nil, fmt.Errorf("encryption: read entropy: %w", err)
	}
	return k, nil
}
