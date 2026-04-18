package encryption

import (
	"crypto/rand"
	"fmt"
	"io"
)

const (
	aeadNonceSize = 12
	aeadTagSize   = 16
	aeadMinLen    = aeadNonceSize + aeadTagSize
)

// Seal encrypts plaintext under dataKey with AES-256-GCM, prepending a
// fresh random 12-byte nonce. Output layout: nonce (12B) || ciphertext
// || GCM tag (16B).
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
	nonce := make([]byte, aeadNonceSize)
	if _, err := io.ReadFull(r, nonce); err != nil {
		return nil, fmt.Errorf("encryption: read nonce entropy: %w", err)
	}
	sealed := aesGCMSeal(dataKey, nonce, plaintext)
	out := make([]byte, 0, len(nonce)+len(sealed))
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

// Open decrypts ciphertext produced by Seal.
func Open(ciphertext, dataKey []byte) ([]byte, error) {
	if len(dataKey) != DataKeySize {
		return nil, fmt.Errorf("encryption: data key %s, want %d", safe(dataKey), DataKeySize)
	}
	if len(ciphertext) < aeadMinLen {
		return nil, ErrDecrypt
	}
	nonce := ciphertext[:aeadNonceSize]
	body := ciphertext[aeadNonceSize:]
	plaintext, err := aesGCMOpen(dataKey, nonce, body)
	if err != nil {
		return nil, errDecryptGCM(err)
	}
	return plaintext, nil
}
