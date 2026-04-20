package encryption

// Keyring is the in-memory, parsed form of <root>/recipients.age.
// It carries the authoritative recipient set, the monotonic
// version counter, the per-repo UUID, and (optionally) the local
// identity that produced the decryption.
//
// Keyrings are immutable after construction. Callers that need a
// different recipient set (e.g. after `rela keys add`) build a new
// RecipientsFile, write it, and reload.
type Keyring struct {
	file *RecipientsFile

	// sortedNames / sortedRecipients mirror file.Recipients in
	// stable alphabetical order so age.Encrypt produces identical
	// stanza ordering across runs.
	sortedNames      []string
	sortedRecipients []Recipient

	// identity is the loaded private identity, or nil if none.
	identity Identity

	// localName is the recipient name corresponding to identity,
	// or "" when identity is absent or doesn't match any listed
	// recipient.
	localName string
}

// LoadKeyring reads <recipientsPath> with identity and returns a
// Keyring. identity is required — reading recipients.age inherently
// needs to unseal, so a caller without a key cannot usefully
// construct a Keyring.
//
// A missing recipients file surfaces as os.ErrNotExist from the
// underlying os.ReadFile; callers distinguish that case to tell
// "no encryption configured" from "encryption is on but something
// is wrong".
func LoadKeyring(recipientsPath string, identity Identity) (*Keyring, error) {
	if identity == nil {
		return nil, ErrNoPrivateKey
	}
	rf, err := ReadRecipientsFile(recipientsPath, identity)
	if err != nil {
		return nil, err
	}

	kr := &Keyring{file: rf, identity: identity}

	recipients, err := rf.RecipientList()
	if err != nil {
		return nil, err
	}
	// RecipientList returns entries in name-sorted order; mirror
	// that ordering locally so Recipients() is O(1) without
	// re-sorting on every call.
	kr.sortedNames = make([]string, 0, len(rf.Recipients))
	for n := range rf.Recipients {
		kr.sortedNames = append(kr.sortedNames, n)
	}
	sortStrings(kr.sortedNames)
	kr.sortedRecipients = recipients

	kr.localName = matchLocalName(rf.Recipients, identity)
	return kr, nil
}

// matchLocalName returns the recipient name whose public key matches
// identity's public recipient, or "" if none match.
func matchLocalName(recipients map[string]string, identity Identity) string {
	pub := identity.PublicRecipient().String()
	for name, r := range recipients {
		if r == pub {
			return name
		}
	}
	return ""
}

// Recipients returns all loaded recipients in deterministic
// name-sorted order. Callers must not mutate the returned slice.
func (kr *Keyring) Recipients() []Recipient { return kr.sortedRecipients }

// RecipientNames returns the sorted list of recipient identity
// names. Callers must not mutate the returned slice.
func (kr *Keyring) RecipientNames() []string { return kr.sortedNames }

// Recipient returns the recipient registered under name, if any.
func (kr *Keyring) Recipient(name string) (Recipient, bool) {
	r, ok := kr.file.Recipients[name]
	if !ok {
		return nil, false
	}
	parsed, err := ParseRecipient(r)
	if err != nil {
		// Should be impossible — Validate / RecipientList already
		// parsed every entry successfully at load time. Guard
		// anyway so a future metadata change doesn't silently
		// swallow errors here.
		return nil, false
	}
	return parsed, true
}

// Identity returns the loaded local identity, or nil.
func (kr *Keyring) Identity() Identity { return kr.identity }

// HasIdentity reports whether a local identity was loaded.
// Retained for API compatibility with the pre-S2 Keyring; callers
// now generally assume Identity is non-nil since LoadKeyring
// requires it.
func (kr *Keyring) HasIdentity() bool { return kr.identity != nil }

// LocalName returns the recipient name whose public key corresponds
// to the loaded identity, or "" when the loaded identity isn't in
// the recipient set.
func (kr *Keyring) LocalName() string { return kr.localName }

// Version returns the monotonic repo-encryption version from the
// loaded recipients file. Every data file sealed under this keyring
// stamps this value into its X-Rela-Version header.
func (kr *Keyring) Version() int { return kr.file.Version }

// RepoID returns the per-repo UUID generated at `rela keys init`.
// Used as the key for per-machine state (last-seen-version,
// in-flight reseal sentinel) so different rela projects don't
// collide in the same XDG state directory.
func (kr *Keyring) RepoID() string { return kr.file.RepoID }

// File returns a pointer to the underlying RecipientsFile for
// callers that need to modify and re-seal it (keys add / remove).
// The returned struct is shared, not copied — mutation affects this
// keyring's state, but since keyrings are discarded after recipient
// changes anyway the caller is expected to build a fresh one.
func (kr *Keyring) File() *RecipientsFile { return kr.file }
