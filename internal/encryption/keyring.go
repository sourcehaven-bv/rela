package encryption

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const pubSuffix = ".pub"

// Keyring holds loaded recipient public keys and, optionally, a local
// private keypair. The private keypair is never exposed; callers
// decrypt through Keyring.Unwrap.
type Keyring struct {
	recipients map[string]*PublicKey
	private    *Keypair
}

// LoadKeyring loads recipients from keysDir and, if privateKeyPath is
// non-empty, the local private key from that path.
//
// keysDir is walked non-recursively. Files ending in ".pub" are parsed
// as recipient public keys; the filename without the suffix is the
// identity. Other entries are skipped.
//
// If privateKeyPath is non-empty but the file is missing, an error is
// returned — an explicit path should resolve. To indicate "no private
// key," pass the empty string.
func LoadKeyring(keysDir, privateKeyPath string) (*Keyring, error) {
	kr := &Keyring{recipients: make(map[string]*PublicKey)}

	if keysDir != "" {
		entries, err := os.ReadDir(keysDir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("encryption: read keys dir: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, pubSuffix) {
				continue
			}
			identity := strings.TrimSuffix(name, pubSuffix)
			path := filepath.Join(keysDir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("encryption: read %s: %w", name, err)
			}
			pub, err := ParsePublicKeyPEM(data)
			if err != nil {
				return nil, errBadPEM(name, err)
			}
			kr.recipients[identity] = pub
		}
	}

	if privateKeyPath != "" {
		data, err := os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("encryption: read private key: %w", err)
		}
		priv, err := ParsePrivateKeyPEM(data)
		if err != nil {
			return nil, errBadPEM(filepath.Base(privateKeyPath), err)
		}
		kr.private = priv
	}

	return kr, nil
}

// Recipient looks up a recipient public key by identity.
func (kr *Keyring) Recipient(id string) (*PublicKey, bool) {
	p, ok := kr.recipients[id]
	return p, ok
}

// Identities returns the recipient identities in sorted order.
func (kr *Keyring) Identities() []string {
	ids := make([]string, 0, len(kr.recipients))
	for id := range kr.recipients {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// HasPrivateKey reports whether a local private key was loaded.
func (kr *Keyring) HasPrivateKey() bool {
	return kr.private != nil
}

// Unwrap decrypts a wrapped data key using the loaded private key.
// Returns ErrNoPrivateKey when no private key is loaded.
func (kr *Keyring) Unwrap(wrapped []byte) ([]byte, error) {
	if kr.private == nil {
		return nil, ErrNoPrivateKey
	}
	return UnwrapKey(wrapped, kr.private)
}
