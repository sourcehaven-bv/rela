package encryption

import (
	"crypto/ecdh"
	"crypto/mlkem"
	"encoding/pem"
	"errors"
	"fmt"
)

const (
	pemTypePrivateV1 = "RELA X25519-MLKEM768 PRIVATE KEY V1"
	pemTypePublicV1  = "RELA X25519-MLKEM768 PUBLIC KEY V1"
	pemTypeWrappedV1 = "RELA WRAPPED KEY V1"

	privatePayloadSize = x25519KeySize + mlkemSeedSize  // 32 + 64 = 96
	publicPayloadSize  = x25519KeySize + mlkemEncapSize // 32 + 1184 = 1216
)

// MarshalPrivateKeyPEM encodes a keypair in PEM form. The payload is
// the X25519 scalar (32B) concatenated with the ML-KEM-768 seed (64B).
// The seed lets the decapsulation key be reconstructed deterministically.
func MarshalPrivateKeyPEM(k *Keypair) ([]byte, error) {
	if k == nil {
		return nil, errors.New("encryption: nil keypair")
	}
	xBytes := k.x25519.Bytes()
	mustLen("x25519 scalar Bytes()", len(xBytes), x25519KeySize)
	seed := k.mlkem.Bytes()
	mustLen("ml-kem decapsulation key Bytes() seed", len(seed), mlkemSeedSize)
	payload := make([]byte, 0, privatePayloadSize)
	payload = append(payload, xBytes...)
	payload = append(payload, seed...)
	block := &pem.Block{Type: pemTypePrivateV1, Bytes: payload}
	return pem.EncodeToMemory(block), nil
}

// ParsePrivateKeyPEM decodes the output of MarshalPrivateKeyPEM.
func ParsePrivateKeyPEM(data []byte) (*Keypair, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("%w: no PEM block", ErrBadPEM)
	}
	if block.Type != pemTypePrivateV1 {
		return nil, errBadPEMType(block.Type, pemTypePrivateV1)
	}
	if len(block.Bytes) != privatePayloadSize {
		return nil, errBadPEMLength(len(block.Bytes), privatePayloadSize)
	}
	xPriv := mustStdlibContract(ecdh.X25519().NewPrivateKey(block.Bytes[:x25519KeySize]))
	mPriv := mustStdlibContract(mlkem.NewDecapsulationKey768(block.Bytes[x25519KeySize:]))
	return &Keypair{x25519: xPriv, mlkem: mPriv}, nil
}

// MarshalPublicKeyPEM encodes a public key in PEM form.
func MarshalPublicKeyPEM(p *PublicKey) ([]byte, error) {
	if p == nil {
		return nil, errors.New("encryption: nil public key")
	}
	xBytes := p.x25519.Bytes()
	mustLen("x25519 public key Bytes()", len(xBytes), x25519KeySize)
	mBytes := p.mlkem.Bytes()
	mustLen("ml-kem encapsulation key Bytes()", len(mBytes), mlkemEncapSize)
	payload := make([]byte, 0, publicPayloadSize)
	payload = append(payload, xBytes...)
	payload = append(payload, mBytes...)
	block := &pem.Block{Type: pemTypePublicV1, Bytes: payload}
	return pem.EncodeToMemory(block), nil
}

// ParsePublicKeyPEM decodes the output of MarshalPublicKeyPEM.
func ParsePublicKeyPEM(data []byte) (*PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("%w: no PEM block", ErrBadPEM)
	}
	if block.Type != pemTypePublicV1 {
		return nil, errBadPEMType(block.Type, pemTypePublicV1)
	}
	if len(block.Bytes) != publicPayloadSize {
		return nil, errBadPEMLength(len(block.Bytes), publicPayloadSize)
	}
	xPub := mustStdlibContract(ecdh.X25519().NewPublicKey(block.Bytes[:x25519KeySize]))
	mPub, err := mlkem.NewEncapsulationKey768(block.Bytes[x25519KeySize:])
	if err != nil {
		// ml-kem NewEncapsulationKey768 does structural validation beyond
		// length; a malformed encap key reaches here.
		return nil, fmt.Errorf("%w: mlkem: %s", ErrBadPEM, err.Error())
	}
	return &PublicKey{x25519: xPub, mlkem: mPub}, nil
}
