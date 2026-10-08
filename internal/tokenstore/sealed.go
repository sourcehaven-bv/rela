package tokenstore

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/state"
)

// KeySize is the length of a token encryption key: AES-256.
const KeySize = 32

const (
	// sealVersion is the first byte of a sealed record. A new layout gets a
	// new version, so an old record is recognized rather than misread.
	sealVersion byte = 1
	keyIDSize        = 8
	nonceSize        = 12
	headerSize       = 1 + keyIDSize + nonceSize
)

// Sealed is a [Store] that encrypts each token with AES-256-GCM into a
// state.KV, under the key `tokens/<name>`.
//
// A record is: version byte, an 8-byte key id, a 12-byte random nonce, then
// the ciphertext. The key id is an HMAC of the key, so a record sealed under
// another key reports [ErrWrongKey] rather than [ErrCorrupt]: an operator
// who rotated token_key learns that, instead of suspecting tampering.
//
// The additional data bound into each record is
// `rela-token|v1|<scope>|<name>`. A record copied to another name, or to
// another tenant schema or project sharing the key, fails to open.
type Sealed struct {
	kv    state.KV
	aead  cipher.AEAD
	keyID [keyIDSize]byte
	scope string
}

// NewSealed returns a store sealing into kv with key, bound to scope. scope
// identifies the tenant: the postgres schema, or the project id on sqlite.
//
// Nil: a nil kv is rejected; there is no store to seal into.
func NewSealed(kv state.KV, key []byte, scope string) (*Sealed, error) {
	if kv == nil {
		return nil, errors.New("tokenstore: NewSealed needs a state store")
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("tokenstore: token_key must be %d bytes, got %d", KeySize, len(key))
	}
	if scope == "" || strings.ContainsAny(scope, "|\x00") {
		return nil, fmt.Errorf("tokenstore: invalid scope %q", scope)
	}
	block, err := aes.NewCipher(key)
	if err != nil { // coverage-ignore: aes.NewCipher fails only on a bad key length, checked above
		return nil, fmt.Errorf("tokenstore: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil { // coverage-ignore: cipher.NewGCM fails only on a non-128-bit block cipher
		return nil, fmt.Errorf("tokenstore: %w", err)
	}
	s := &Sealed{kv: kv, aead: aead, scope: scope}
	s.keyID = KeyID(key)
	return s, nil
}

// KeyID returns the 8-byte id of key that a sealed record carries.
func KeyID(key []byte) [keyIDSize]byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("rela-token-key-id"))
	var id [keyIDSize]byte
	copy(id[:], mac.Sum(nil))
	return id
}

func (s *Sealed) aad(name Name) []byte {
	return []byte("rela-token|v1|" + s.scope + "|" + string(name))
}

// Get implements [Store].
func (s *Sealed) Get(ctx context.Context, name Name) (Token, error) {
	raw, err := s.kv.Get(ctx, name.stateKey())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return Token{}, fmt.Errorf("%w for %q", ErrNotFound, name)
		}
		return Token{}, fmt.Errorf("tokenstore: read %q: %w", name, err)
	}
	if len(raw) < headerSize || raw[0] != sealVersion {
		return Token{}, fmt.Errorf("%w (%q: unknown record layout)", ErrCorrupt, name)
	}
	if !bytes.Equal(raw[1:1+keyIDSize], s.keyID[:]) {
		return Token{}, fmt.Errorf("%w (%q)", ErrWrongKey, name)
	}
	nonce := raw[1+keyIDSize : headerSize]
	plain, err := s.aead.Open(nil, nonce, raw[headerSize:], s.aad(name))
	if err != nil {
		return Token{}, fmt.Errorf("%w (%q: does not decrypt for this name and scope)", ErrCorrupt, name)
	}
	var t Token
	if err := json.Unmarshal(plain, &t); err != nil || t.Refresh == "" {
		return Token{}, fmt.Errorf("%w (%q: does not decode)", ErrCorrupt, name)
	}
	return t, nil
}

// Put implements [Store].
func (s *Sealed) Put(ctx context.Context, name Name, t Token) error {
	if t.Refresh == "" {
		return errors.New("tokenstore: a stored token needs a refresh token")
	}
	plain, err := json.Marshal(t)
	if err != nil { // coverage-ignore: a Token of strings and times always marshals
		return fmt.Errorf("tokenstore: encode %q: %w", name, err)
	}
	out := make([]byte, headerSize, headerSize+len(plain)+s.aead.Overhead())
	out[0] = sealVersion
	copy(out[1:], s.keyID[:])
	if _, err := rand.Read(out[1+keyIDSize : headerSize]); err != nil { // coverage-ignore: crypto/rand does not fail
		return fmt.Errorf("tokenstore: nonce: %w", err)
	}
	out = s.aead.Seal(out, out[1+keyIDSize:headerSize], plain, s.aad(name))
	if err := s.kv.Put(ctx, name.stateKey(), out); err != nil {
		return fmt.Errorf("tokenstore: write %q: %w", name, err)
	}
	return nil
}

// Delete implements [Store].
func (s *Sealed) Delete(ctx context.Context, name Name) error {
	if err := s.kv.Delete(ctx, name.stateKey()); err != nil {
		return fmt.Errorf("tokenstore: delete %q: %w", name, err)
	}
	return nil
}
