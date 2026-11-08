// Package peerkey issues and verifies peer keys, the credentials a sync peer
// presents to a source rela (DEC-81161C3, FEAT-CMVZQFI).
//
// A peer key is [Prefix] followed by 32 random bytes in unpadded base64url.
// The source stores only its [Digest], the SHA-256 of the whole key, under
// `peer_key.<name>` in `.rela/secrets.yaml`. A request that presents a key
// whose digest is listed authenticates as the peer `peer:<name>`.
//
// A slow password hash is not needed: the key carries 256 bits of entropy,
// so a leaked digest cannot be reversed by guessing. The digest is looked up
// in a map. A timing difference in that lookup tells an attacker only about
// digests of keys it already holds, never about a key it does not.
//
// The prefix lets the request edge tell a peer key from a JWT in the same
// header, and lets a secret scanner find a leaked key.
package peerkey

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Prefix starts every peer key.
const Prefix = "relapeer_"

// keyBytes is the random part of a key.
const keyBytes = 32

// encodedLen is the length of the random part in unpadded base64url.
var encodedLen = base64.RawURLEncoding.EncodedLen(keyBytes)

// digestScheme starts every stored digest.
const digestScheme = "sha256:"

// ErrMalformed reports a credential with the peer key prefix whose rest is
// not a well-formed key.
var ErrMalformed = errors.New("peerkey: malformed key")

// ErrUnknown reports a well-formed key whose digest is not listed. A revoked
// key and a key that never existed look the same.
var ErrUnknown = errors.New("peerkey: unknown key")

// IsPeerKey reports whether credential is presented as a peer key: it starts
// with [Prefix]. Whether it is well formed is for [Keyring.Verify] to decide,
// so a malformed key is refused rather than passed to another identity
// source.
func IsPeerKey(credential string) bool { return strings.HasPrefix(credential, Prefix) }

// Generate returns a new key and its digest.
//
// Nil: rejected — r is required so tests can pin the bytes; production passes
// crypto/rand.Reader.
func Generate(r io.Reader) (key, digest string, err error) {
	if r == nil {
		return "", "", errors.New("peerkey: Generate: nil random source")
	}
	buf := make([]byte, keyBytes)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", "", fmt.Errorf("peerkey: read random bytes: %w", err)
	}
	key = Prefix + base64.RawURLEncoding.EncodeToString(buf)
	return key, Digest(key), nil
}

// Digest returns the stored form of key: "sha256:" and the lowercase hex
// SHA-256 of the whole key, prefix included.
func Digest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return digestScheme + hex.EncodeToString(sum[:])
}

// parseDigest reads the stored form [Digest] writes.
func parseDigest(s string) ([sha256.Size]byte, error) {
	var out [sha256.Size]byte
	h, ok := strings.CutPrefix(strings.TrimSpace(s), digestScheme)
	if !ok {
		return out, fmt.Errorf("the value must start with %q", digestScheme)
	}
	if len(h) != hex.EncodedLen(sha256.Size) || strings.ToLower(h) != h {
		return out, fmt.Errorf("the value must be %q and %d lowercase hex digits", digestScheme,
			hex.EncodedLen(sha256.Size))
	}
	if _, err := hex.Decode(out[:], []byte(h)); err != nil {
		return out, fmt.Errorf("the value must be %q and %d lowercase hex digits", digestScheme,
			hex.EncodedLen(sha256.Size))
	}
	return out, nil
}

// Keyring maps key digests to peer names. The zero value lists no key.
type Keyring struct {
	byDigest map[[sha256.Size]byte]string
}

// NewKeyring builds a keyring from peer name to stored digest. Every name
// must pass [principal.ValidPeerName] and every digest must parse; two names
// with one digest are refused, since the key could not say which peer it is.
// An error names the entry, never its value.
func NewKeyring(entries map[string]string) (*Keyring, error) {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	k := &Keyring{byDigest: make(map[[sha256.Size]byte]string, len(entries))}
	for _, name := range names {
		if !principal.ValidPeerName(name) {
			return nil, fmt.Errorf("peer_key.%s: a peer name is 1-63 lowercase letters, digits, '_' or '-', "+
				"starting with a letter or digit", name)
		}
		d, err := parseDigest(entries[name])
		if err != nil {
			return nil, fmt.Errorf("peer_key.%s: %w", name, err)
		}
		if other, dup := k.byDigest[d]; dup {
			return nil, fmt.Errorf("peer_key.%s and peer_key.%s hold the same digest", other, name)
		}
		k.byDigest[d] = name
	}
	return k, nil
}

// Len returns the number of keys.
func (k *Keyring) Len() int {
	if k == nil {
		return 0
	}
	return len(k.byDigest)
}

// Verify returns the peer name key authenticates. A credential without
// [Prefix], or with a rest that is not 32 bytes of unpadded base64url, is
// [ErrMalformed]; a well-formed key that is not listed is [ErrUnknown].
//
// Nil: a nil Keyring is accepted and lists no key.
func (k *Keyring) Verify(key string) (string, error) {
	body, ok := strings.CutPrefix(key, Prefix)
	if !ok || len(body) != encodedLen {
		return "", ErrMalformed
	}
	if _, err := base64.RawURLEncoding.Strict().DecodeString(body); err != nil {
		return "", ErrMalformed
	}
	if k == nil {
		return "", ErrUnknown
	}
	name, ok := k.byDigest[sha256.Sum256([]byte(key))]
	if !ok {
		return "", ErrUnknown
	}
	return name, nil
}
