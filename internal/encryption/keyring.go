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

// Keyring holds the set of recipient public keys for a rela repo and,
// optionally, the local identity used to unseal blobs addressed to us.
type Keyring struct {
	// recipients maps identity-name ("alice", "bob") to Recipient.
	// The name is the filename stem of "<name>.pub" under keysDir.
	recipients map[string]Recipient

	// identity is the loaded private identity, or nil if none.
	identity Identity

	// localName is the filename-stem whose recipient public key
	// matches identity. Empty when identity is nil OR when identity
	// doesn't correspond to any listed recipient. UX affordance only.
	localName string
}

// LoadKeyring loads recipient pubkeys from every "<name>.pub" file in
// keysDir, and optionally the local identity from identityPath. A
// missing keysDir is treated as an empty recipient set; a missing
// identityPath is treated as "no local identity." A present but
// unreadable or malformed file is a hard error.
//
// Use LoadFromDir for the standard project-root layout.
func LoadKeyring(keysDir, identityPath string) (*Keyring, error) {
	kr := &Keyring{recipients: make(map[string]Recipient)}

	if err := loadRecipients(kr, keysDir); err != nil {
		return nil, err
	}

	if identityPath != "" {
		f, err := os.Open(identityPath)
		if err != nil {
			return nil, fmt.Errorf("encryption: open identity %s: %w", identityPath, err)
		}
		defer f.Close()
		id, err := ReadIdentity(f)
		if err != nil {
			return nil, fmt.Errorf("encryption: %s: %w", filepath.Base(identityPath), err)
		}
		kr.identity = id
		kr.localName = matchLocalName(kr.recipients, id)
	}

	return kr, nil
}

func loadRecipients(kr *Keyring, keysDir string) error {
	if keysDir == "" {
		return nil
	}
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("encryption: read keys dir: %w", err)
	}
	for _, e := range entries {
		if err := loadOneRecipient(kr, keysDir, e); err != nil {
			return err
		}
	}
	return nil
}

func loadOneRecipient(kr *Keyring, keysDir string, e os.DirEntry) error {
	if e.IsDir() {
		return nil
	}
	name := e.Name()
	if !strings.HasSuffix(name, pubSuffix) {
		return nil
	}
	stem := strings.TrimSuffix(name, pubSuffix)
	if _, dup := kr.recipients[stem]; dup {
		return fmt.Errorf("encryption: duplicate recipient identity %q", stem)
	}
	data, err := os.ReadFile(filepath.Join(keysDir, name))
	if err != nil {
		return fmt.Errorf("encryption: read %s: %w", name, err)
	}
	// Each .pub file contains one age recipient as a single line
	// (possibly with a comment line above it, which ParseRecipient
	// cannot handle). Trim to the first non-empty non-comment line.
	line := firstContentLine(data)
	r, err := ParseRecipient(line)
	if err != nil {
		return fmt.Errorf("encryption: %s: %w", name, err)
	}
	kr.recipients[stem] = r
	return nil
}

// firstContentLine returns the first non-empty, non-comment line of b.
// Matches age's ParseRecipients convention so the same .pub files work
// with the age CLI.
func firstContentLine(b []byte) string {
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line
	}
	return ""
}

// matchLocalName returns the recipient name whose public key equals
// identity's public recipient, or "" if none match.
func matchLocalName(recipients map[string]Recipient, identity Identity) string {
	pub := identity.PublicRecipient().String()
	for name, r := range recipients {
		if r.String() == pub {
			return name
		}
	}
	return ""
}

// Recipients returns all loaded recipients (public keys). Order is
// sorted by name so callers that feed this to age.Encrypt get
// deterministic output.
func (kr *Keyring) Recipients() []Recipient {
	names := kr.RecipientNames()
	out := make([]Recipient, 0, len(names))
	for _, n := range names {
		out = append(out, kr.recipients[n])
	}
	return out
}

// RecipientNames returns the sorted list of recipient identity names.
func (kr *Keyring) RecipientNames() []string {
	names := make([]string, 0, len(kr.recipients))
	for n := range kr.recipients {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Recipient returns the recipient registered under name, if any.
func (kr *Keyring) Recipient(name string) (Recipient, bool) {
	r, ok := kr.recipients[name]
	return r, ok
}

// Identity returns the loaded local identity, or nil.
func (kr *Keyring) Identity() Identity { return kr.identity }

// HasIdentity reports whether a local identity was loaded.
func (kr *Keyring) HasIdentity() bool { return kr.identity != nil }

// LocalName returns the recipient name whose public key corresponds
// to the loaded identity, or "" when either no identity is loaded or
// the loaded identity isn't in the recipient set.
func (kr *Keyring) LocalName() string { return kr.localName }
