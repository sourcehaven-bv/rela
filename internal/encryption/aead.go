package encryption

import (
	"crypto/rand"
	"fmt"
	"io"
)

// Seal encrypts plaintext under dataKey with AES-256-GCM, prepending a
// fresh random nonce. Output layout: nonce || ciphertext || GCM tag.
//
// Nonce and tag sizes come from cipher.AEAD — 12 bytes and 16 bytes
// respectively for the stdlib GCM AEAD. See newGCM.
//
// Callers MUST NOT reuse a data key beyond 2^32 Seal calls (AES-GCM
// birthday bound). Because NewDataKey is cheap, the intended pattern is
// one data key per encrypted artifact — at which point the bound is
// unreachable.
func Seal(plaintext, dataKey []byte) ([]byte, error) {
	return sealWith(rand.Reader, plaintext, dataKey)
}

func sealWith(r io.Reader, plaintext, dataKey []byte) ([]byte, error) {
	if len(dataKey) != DataKeySize {
		return nil, fmt.Errorf("encryption: data key %s, want %d", safe(dataKey), DataKeySize)
	}
	aead := newGCM(dataKey)
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(r, nonce); err != nil {
		return nil, fmt.Errorf("encryption: read nonce entropy: %w", err)
	}
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts ciphertext produced by Seal.
func Open(ciphertext, dataKey []byte) ([]byte, error) {
	if len(dataKey) != DataKeySize {
		return nil, fmt.Errorf("encryption: data key %s, want %d", safe(dataKey), DataKeySize)
	}
	aead := newGCM(dataKey)
	if len(ciphertext) < aead.NonceSize()+aead.Overhead() {
		return nil, ErrDecrypt
	}
	nonce := ciphertext[:aead.NonceSize()]
	body := ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, errDecryptGCM(err)
	}
	return plaintext, nil
}
