package encryption

import (
	"bytes"
	"crypto/ecdh"
	"crypto/mlkem"
	"crypto/rand"
	"fmt"
	"io"
)

const (
	x25519KeySize   = 32
	mlkemSeedSize   = mlkem.SeedSize // 64
	mlkemEncapSize  = mlkem.EncapsulationKeySize768
	mlkemCipherSize = mlkem.CiphertextSize768
)

// Keypair is a hybrid X25519 + ML-KEM-768 private key. It does not
// implement String, GoString, or MarshalJSON: the default fmt verbs
// render the struct without revealing secret scalars.
type Keypair struct {
	x25519 *ecdh.PrivateKey
	mlkem  *mlkem.DecapsulationKey768
}

// PublicKey is the recipient-facing half of a hybrid keypair.
type PublicKey struct {
	x25519 *ecdh.PublicKey
	mlkem  *mlkem.EncapsulationKey768
}

// GenerateKeypair returns a fresh hybrid keypair using cryptographic
// randomness.
func GenerateKeypair() (*Keypair, error) {
	return generateKeypair(rand.Reader)
}

func generateKeypair(r io.Reader) (*Keypair, error) {
	xSeed := make([]byte, x25519KeySize)
	if _, err := io.ReadFull(r, xSeed); err != nil {
		return nil, fmt.Errorf("encryption: read x25519 entropy: %w", err)
	}
	xPriv := mustStdlibContract(ecdh.X25519().NewPrivateKey(xSeed))

	mSeed := make([]byte, mlkemSeedSize)
	if _, err := io.ReadFull(r, mSeed); err != nil {
		return nil, fmt.Errorf("encryption: read mlkem entropy: %w", err)
	}
	mPriv := mustStdlibContract(mlkem.NewDecapsulationKey768(mSeed))

	return &Keypair{x25519: xPriv, mlkem: mPriv}, nil
}

// PublicKey returns the recipient-facing half of the keypair.
func (k *Keypair) PublicKey() *PublicKey {
	return &PublicKey{
		x25519: k.x25519.PublicKey(),
		mlkem:  k.mlkem.EncapsulationKey(),
	}
}

// equal reports whether two public keys encode to the same bytes.
// Used by Keyring to identify which stored recipient corresponds to
// the local private key — no user-visible API change.
func (p *PublicKey) equal(other *PublicKey) bool {
	if p == nil || other == nil {
		return false
	}
	if !bytes.Equal(p.x25519.Bytes(), other.x25519.Bytes()) {
		return false
	}
	return bytes.Equal(p.mlkem.Bytes(), other.mlkem.Bytes())
}
