package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// hkdfInfoV1 is the KDF context for the V1 hybrid envelope. It is
// inspired by RFC 9180 §4.1 LabeledExtract/LabeledExpand but is a
// simplified construction — not HPKE proper. A future swap bumps the
// version and the info string together.
const hkdfInfoV1 = "rela-encryption v1"

// Blob format is a wire contract — offsets must be compile-time
// constants. The AES-GCM sizes (12-byte nonce, 16-byte tag) are the
// stdlib defaults returned by cipher.AEAD.NonceSize() / Overhead() for
// GCM. They're hardcoded here because the blob layout fixes them at
// V1. A future algorithm swap gets a new magic/version and can use
// different numbers.
//
// Stdlib references:
//   - https://pkg.go.dev/crypto/cipher#NewGCM           (NonceSize = 12)
//   - https://pkg.go.dev/crypto/cipher#AEAD.Overhead    (tag = 16)
const (
	wrapMagic    = "RLAE"
	wrapMagicLen = 4
	wrapVersion  = 0x01
	wrapKEKSize  = 32

	wrapNonceSize  = 12 // cipher.AEAD.NonceSize() for GCM
	wrapGCMTagSize = 16 // cipher.AEAD.Overhead() for GCM

	wrappedBlobSize = wrapMagicLen + 1 + x25519KeySize + mlkemCipherSize + (DataKeySize + wrapGCMTagSize)

	wrapOffsetVersion = wrapMagicLen
	wrapOffsetEphPub  = wrapOffsetVersion + 1
	wrapOffsetMLKEMCt = wrapOffsetEphPub + x25519KeySize
	wrapOffsetWrapped = wrapOffsetMLKEMCt + mlkemCipherSize
)

// WrapKey encrypts a 32-byte data key for a single recipient. The
// output is a self-describing, fixed-size 1173-byte blob.
//
// V1 fields are every fixed-size, so the format carries no length
// prefixes. A future algorithm change gets a new magic/version.
func WrapKey(dataKey []byte, recipient *PublicKey) ([]byte, error) {
	return wrapKey(rand.Reader, dataKey, recipient)
}

func wrapKey(r io.Reader, dataKey []byte, recipient *PublicKey) ([]byte, error) {
	if recipient == nil {
		return nil, errors.New("encryption: nil recipient")
	}
	if len(dataKey) != DataKeySize {
		return nil, fmt.Errorf("encryption: data key %s, want %d", safe(dataKey), DataKeySize)
	}

	ephSeed := make([]byte, x25519KeySize)
	if _, err := io.ReadFull(r, ephSeed); err != nil {
		return nil, fmt.Errorf("encryption: read ephemeral entropy: %w", err)
	}
	ephPriv := mustStdlibContract(ecdh.X25519().NewPrivateKey(ephSeed))
	// ECDH against a recipient public key can in principle fail for
	// adversarial inputs (low-order points). A recipient loaded via
	// ParsePublicKeyPEM has been structurally validated, but we still
	// treat this as a real error path.
	xShared, err := ephPriv.ECDH(recipient.x25519)
	if err != nil {
		return nil, fmt.Errorf("encryption: x25519 ecdh: %w", err)
	}

	mShared, mCt := recipient.mlkem.Encapsulate()

	kek := deriveKEK(xShared, mShared)

	nonce := make([]byte, wrapNonceSize) // all-zero: KEK is single-use per wrap
	wrapped := aesGCMSeal(kek, nonce, dataKey)

	out := make([]byte, 0, wrappedBlobSize)
	out = append(out, wrapMagic...)
	out = append(out, wrapVersion)
	out = append(out, ephPriv.PublicKey().Bytes()...)
	out = append(out, mCt...)
	out = append(out, wrapped...)
	return out, nil
}

// UnwrapKey recovers a data key from a blob produced by WrapKey using
// the matching private keypair.
func UnwrapKey(wrapped []byte, k *Keypair) ([]byte, error) {
	if k == nil {
		return nil, errors.New("encryption: nil keypair")
	}
	if len(wrapped) != wrappedBlobSize {
		return nil, errBadBlobLength(len(wrapped))
	}
	if string(wrapped[:wrapMagicLen]) != wrapMagic {
		return nil, errBadBlobMagic()
	}
	if wrapped[wrapOffsetVersion] != wrapVersion {
		return nil, errBadBlobVersion(wrapped[wrapOffsetVersion])
	}

	ephPub := mustStdlibContract(ecdh.X25519().NewPublicKey(wrapped[wrapOffsetEphPub:wrapOffsetMLKEMCt]))
	// x25519 ECDH can return an error for adversarial (low-order) peer
	// points. Reachable with a crafted malicious blob.
	xShared, err := k.x25519.ECDH(ephPub)
	if err != nil {
		return nil, fmt.Errorf("%w: x25519 ecdh: %s", ErrBadBlob, err.Error())
	}

	mShared := mustStdlibContract(k.mlkem.Decapsulate(wrapped[wrapOffsetMLKEMCt:wrapOffsetWrapped]))

	kek := deriveKEK(xShared, mShared)

	nonce := make([]byte, wrapNonceSize)
	dataKey, err := aesGCMOpen(kek, nonce, wrapped[wrapOffsetWrapped:])
	if err != nil {
		return nil, errDecryptGCM(err)
	}
	return dataKey, nil
}

func deriveKEK(x25519Shared, mlkemShared []byte) []byte {
	ikm := make([]byte, 0, len(x25519Shared)+len(mlkemShared))
	ikm = append(ikm, x25519Shared...)
	ikm = append(ikm, mlkemShared...)
	return mustStdlibContract(hkdf.Key(sha256.New, ikm, nil, hkdfInfoV1, wrapKEKSize))
}

func aesGCMSeal(key, nonce, plaintext []byte) []byte {
	aead := newGCM(key)
	return aead.Seal(nil, nonce, plaintext, nil)
}

// aesGCMOpen returns the plaintext on success, or an error on auth
// failure. Used for both blob unwrap (auth failure = crafted blob) and
// Open (auth failure = wrong key or tampered ciphertext).
func aesGCMOpen(key, nonce, ciphertext []byte) ([]byte, error) {
	aead := newGCM(key)
	return aead.Open(nil, nonce, ciphertext, nil)
}

func newGCM(key []byte) cipher.AEAD {
	block := mustStdlibContract(aes.NewCipher(key))
	aead := mustStdlibContract(cipher.NewGCM(block))
	// Guard against a hypothetical future stdlib change to GCM
	// defaults. Our wire format hardcodes these sizes.
	mustLen("cipher.AEAD GCM NonceSize()", aead.NonceSize(), wrapNonceSize)
	mustLen("cipher.AEAD GCM Overhead()", aead.Overhead(), wrapGCMTagSize)
	return aead
}
