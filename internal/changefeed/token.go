package changefeed

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// tokenAAD binds a token to this format. A future format changes the string,
// so an old token fails to open and the client resyncs.
const tokenAAD = "rela-changefeed-v1" //nolint:gosec // G101: a format label, not a credential

// Token layout: a GCM nonce, the sequence number, and the GCM tag.
const (
	nonceSize = 12
	seqSize   = 8
	tagSize   = 16
)

// tokenLen is the encoded length of every token, in unpadded base64url.
var tokenLen = base64.RawURLEncoding.EncodedLen(nonceSize + seqSize + tagSize)

// sealer turns sequence numbers into opaque resume tokens and back.
//
// A token is encrypted rather than signed so that it does not reveal how many
// changes happened between two tokens: a principal sees tokens on frames for
// the types it can read, and a readable counter would count writes to every
// other type too. The key is random per process, so a token from another
// process or an earlier run fails to open, which is how a stale position is
// detected. A random 96-bit nonce is safe for far more tokens (2^32) than one
// process mints.
type sealer struct {
	aead cipher.AEAD
}

func newSealer() (*sealer, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("changefeed: generating token key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("changefeed: token cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("changefeed: token cipher: %w", err)
	}
	return &sealer{aead: aead}, nil
}

func (s *sealer) seal(seq uint64) string {
	nonce := make([]byte, s.aead.NonceSize(), s.aead.NonceSize()+seqSize+s.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		// crypto/rand does not fail on supported platforms; reusing a nonce
		// would break GCM, so refuse to mint rather than continue.
		panic(fmt.Sprintf("changefeed: reading nonce: %v", err))
	}
	var plain [seqSize]byte
	binary.BigEndian.PutUint64(plain[:], seq)
	return base64.RawURLEncoding.EncodeToString(s.aead.Seal(nonce, nonce, plain[:], []byte(tokenAAD)))
}

// open returns the sequence number in token. It reports false for anything
// this sealer did not mint, including tokens of the wrong length.
func (s *sealer) open(token string) (uint64, bool) {
	if len(token) != tokenLen {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil {
		return 0, false
	}
	n := s.aead.NonceSize()
	plain, err := s.aead.Open(nil, raw[:n], raw[n:], []byte(tokenAAD))
	if err != nil || len(plain) != seqSize {
		return 0, false
	}
	return binary.BigEndian.Uint64(plain), true
}
