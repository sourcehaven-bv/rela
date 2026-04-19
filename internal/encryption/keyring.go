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
//
// localIdentity is the identity string whose stored public key matches
// the loaded private key, or "" when either side is absent or no
// recipient matches. Computed once at load; used by consumers to
// attempt only the wrap that should work and surface tamper/corruption
// errors verbatim (instead of collapsing them into "no matching key").
type Keyring struct {
	recipients    map[string]*PublicKey
	private       *Keypair
	localIdentity string
}

// LoadKeyring loads recipients from keysDir and, if privateKeyPath is
// non-empty, the local private key from that path.
//
// keysDir is walked non-recursively. Files ending in ".pub" are parsed
// as recipient public keys; the filename without the suffix is the
// identity. Other entries are skipped.
//
// Behavior on errors is **fail-fast**: the first unreadable or
// unparseable ".pub" file aborts the load. The intent is that a
// broken recipient file in a shared repo should surface loudly rather
// than silently drop a team member from the recipient set. Duplicate
// identities (e.g., a case-insensitive filesystem yielding both
// "Alice.pub" and "alice.pub") are also an error.
//
// If privateKeyPath is non-empty but the file is missing, an error is
// returned — an explicit path should resolve. To indicate "no private
// key," pass the empty string.
func LoadKeyring(keysDir, privateKeyPath string) (*Keyring, error) {
	kr := &Keyring{recipients: make(map[string]*PublicKey)}

	if err := loadRecipients(kr, keysDir); err != nil {
		return nil, err
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
		kr.localIdentity = matchLocalIdentity(kr.recipients, priv.PublicKey())
	}

	return kr, nil
}

// matchLocalIdentity returns the identity whose stored public key
// equals pub, or "" if none match. A private key that doesn't
// correspond to any listed recipient is still a valid configuration —
// Unwrap will simply return ErrNoMatchingKey.
func matchLocalIdentity(recipients map[string]*PublicKey, pub *PublicKey) string {
	for id, rp := range recipients {
		if rp.equal(pub) {
			return id
		}
	}
	return ""
}

// loadRecipients populates kr.recipients from keysDir. Empty keysDir
// or a missing directory is treated as "no recipients." Broken files
// and duplicate identities fail the load.
func loadRecipients(kr *Keyring, keysDir string) error {
	if keysDir == "" {
		return nil
	}
	entries, err := os.ReadDir(keysDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("encryption: read keys dir: %w", err)
	}
	for _, e := range entries {
		if err := loadRecipient(kr, keysDir, e); err != nil {
			return err
		}
	}
	return nil
}

func loadRecipient(kr *Keyring, keysDir string, e os.DirEntry) error {
	if e.IsDir() {
		return nil
	}
	name := e.Name()
	if !strings.HasSuffix(name, pubSuffix) {
		return nil
	}
	identity := strings.TrimSuffix(name, pubSuffix)
	if _, dup := kr.recipients[identity]; dup {
		return fmt.Errorf("encryption: duplicate recipient identity %q", identity)
	}
	data, err := os.ReadFile(filepath.Join(keysDir, name))
	if err != nil {
		return fmt.Errorf("encryption: read %s: %w", name, err)
	}
	pub, err := ParsePublicKeyPEM(data)
	if err != nil {
		return errBadPEM(name, err)
	}
	kr.recipients[identity] = pub
	return nil
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

// LocalIdentity returns the identity whose stored recipient public
// key corresponds to the loaded private key, or "" when either the
// private key is absent or it doesn't match any known recipient.
func (kr *Keyring) LocalIdentity() string {
	return kr.localIdentity
}

// Unwrap decrypts a wrapped data key using the loaded private key.
// Returns ErrNoPrivateKey when no private key is loaded.
func (kr *Keyring) Unwrap(wrapped []byte) ([]byte, error) {
	if kr.private == nil {
		return nil, ErrNoPrivateKey
	}
	return UnwrapKey(wrapped, kr.private)
}
