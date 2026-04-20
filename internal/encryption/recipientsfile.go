package encryption

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RecipientsFileName is the filename of the authoritative encrypted
// recipient list, committed at the project root. Its presence (not
// .rela/encryption.yaml's presence) is what flips the store into
// encrypted mode in the post-S2 design, though the marker in
// .rela/encryption.yaml is retained during the transition for
// back-compat with callers that haven't been updated yet.
const RecipientsFileName = "recipients.age"

// RecipientsFile is the plaintext payload sealed inside
// <root>/recipients.age. On read the file is decrypted with the
// local identity; the resulting bytes parse as this struct.
//
// Every field is load-bearing:
//
//   - Version is the monotonic repo-wide encryption epoch. It bumps
//     on every recipient-set change (keys add / keys remove). Every
//     sealed data file carries this value in its X-Rela-Version
//     header; readers enforce "observed ≥ last-seen-locally" so
//     rollback of the recipient list or any individual file is
//     detected.
//   - RepoID is a one-time UUID generated at `rela keys init`.
//     Identifies the repo across machines and cloud backups so
//     per-machine state (last-seen-version, in-flight reseal
//     sentinels) can be kept outside the synced tree without
//     colliding across different rela projects.
//   - Recipients maps name → age public-key string. This is the
//     authoritative recipient set — keys/*.pub no longer exists.
//     Adding an unauthorized recipient requires decrypting the
//     current file, so the cloud adversary (without a private key)
//     cannot silently add themselves.
type RecipientsFile struct {
	Version    int               `yaml:"version"`
	RepoID     string            `yaml:"repo_id"`
	Recipients map[string]string `yaml:"recipients"`
}

// Validate checks the RecipientsFile for structural sanity without
// touching the filesystem or crypto. Used after parsing a decrypted
// payload to catch corruption or a schema-mismatched file before
// callers act on it.
func (r *RecipientsFile) Validate() error {
	if r.Version < 1 {
		return fmt.Errorf("encryption: invalid recipients file: version %d < 1", r.Version)
	}
	if r.RepoID == "" {
		return errors.New("encryption: invalid recipients file: repo_id is empty")
	}
	if len(r.Recipients) == 0 {
		return errors.New("encryption: invalid recipients file: no recipients")
	}
	for name, pub := range r.Recipients {
		if name == "" {
			return errors.New("encryption: invalid recipients file: empty recipient name")
		}
		if pub == "" {
			return fmt.Errorf("encryption: invalid recipients file: empty public key for %q", name)
		}
	}
	return nil
}

// RecipientList parses every stored pubkey string into a concrete
// Recipient, sorted by name so the emitted age stanzas are
// deterministic. A single malformed entry fails the whole load.
func (r *RecipientsFile) RecipientList() ([]Recipient, error) {
	names := make([]string, 0, len(r.Recipients))
	for n := range r.Recipients {
		names = append(names, n)
	}
	sortStrings(names)
	out := make([]Recipient, 0, len(names))
	for _, n := range names {
		rec, err := ParseRecipient(r.Recipients[n])
		if err != nil {
			return nil, fmt.Errorf("encryption: recipients file: %s: %w", n, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

// ReadRecipientsFile reads path, unseals it with identity, and
// parses the resulting YAML into a RecipientsFile. Errors surface
// decrypt failures (wrong identity, corrupted / tampered blob)
// through the usual encryption error predicates — IsNoMatchingKey /
// IsCorrupted — so the calling CLI can tell the user whether they're
// using the wrong key or whether the file itself was replaced.
//
// A missing file is returned as os.ErrNotExist so callers can
// distinguish a cleartext repo (file absent) from a load failure.
func ReadRecipientsFile(path string, identity Identity) (*RecipientsFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cleartext, err := Unseal(raw, identity)
	if err != nil {
		return nil, fmt.Errorf("encryption: unseal %s: %w", RecipientsFileName, err)
	}
	var rf RecipientsFile
	if err := yaml.Unmarshal(cleartext, &rf); err != nil {
		return nil, fmt.Errorf("encryption: parse %s: %w", RecipientsFileName, err)
	}
	if err := rf.Validate(); err != nil {
		return nil, err
	}
	return &rf, nil
}

// WriteRecipientsFile serializes rf to YAML, seals it under the
// recipients named in rf.Recipients, and writes it to path
// atomically (temp + rename).
//
// It does not pre-check that the sealed file can be unsealed by any
// *particular* caller — the caller is responsible for ensuring it
// includes itself in rf.Recipients. Omitting yourself from a recipient
// list you're about to seal is a lock-yourself-out operation; the
// CLI guards against it, this function does not.
func WriteRecipientsFile(path string, rf *RecipientsFile) error {
	if err := rf.Validate(); err != nil {
		return err
	}
	recipients, err := rf.RecipientList()
	if err != nil {
		return err
	}
	plaintext, err := yaml.Marshal(rf)
	if err != nil {
		return fmt.Errorf("encryption: marshal recipients file: %w", err)
	}
	sealed, err := Seal(plaintext, recipients)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, sealed, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// NewRepoID generates a fresh per-repo identifier used as the key
// for per-machine state (last-seen-version, reseal sentinel).
// Format: 16 random bytes hex-encoded. Collision across repos is
// statistically impossible.
func NewRepoID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("encryption: generate repo id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// sortStrings is a tiny local helper so this file doesn't depend on
// the sort package for a single call site. Insertion sort is fine
// for the small recipient counts rela ever sees.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
