package encryption

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// resealSentinelFile is the filename of the in-progress re-seal
// sentinel, kept in the same XDG per-repo directory as the
// last-seen-version file. Plaintext YAML — living outside the
// synced tree means the cloud adversary we defend against can't
// see or tamper with it, so encryption would add no security
// benefit and complicate crash recovery (we'd need a key to
// decrypt the sentinel, but the re-seal flow is mid-rotation and
// the key situation is fragile).
const resealSentinelFile = "reseal-progress.yaml"

// ResealSentinel captures the in-flight state of a `rela keys add`
// or `rela keys remove` operation. It is written to XDG state
// BEFORE the data-file walk starts and deleted ONLY after the
// walk, the recipients.age update, and any other on-disk mutations
// complete successfully. If a sentinel is present at
// factory-open time, the previous process crashed mid-rotation;
// recovery resumes the walk using whatever's in the sentinel.
//
// Design decision: the sentinel lives outside the repo, not inside
// .rela/ or at the repo root. This is deliberate and matches the
// last-seen-version file. If the sentinel lived in the synced
// tree, a cloud-side adversary could plant a fake one naming
// attacker-controlled recipients, and the next legitimate rela
// invocation would obediently re-encrypt the whole repo to the
// attacker. Keeping state outside XDG makes that attack require
// local machine compromise.
type ResealSentinel struct {
	// FromVersion is the recipients.age version at the start of
	// the rotation (the version currently on disk unless the
	// recipients.age update already completed).
	FromVersion int `yaml:"from_version"`

	// ToVersion is the version to stamp into every re-sealed
	// file's header and to record in the new recipients.age.
	ToVersion int `yaml:"to_version"`

	// RepoRoot is the absolute path to the rela project. Required
	// because recovery runs from a cold process that knows only
	// the XDG state dir — it has to know which repo to walk.
	RepoRoot string `yaml:"repo_root"`

	// NewRecipients is the recipient list the rotation is
	// migrating TO, stored as age public-key strings. Re-parsed
	// on recovery rather than trusting an in-memory identity map.
	NewRecipients map[string]string `yaml:"new_recipients"`

	// Operation is a human-readable label for the CLI command
	// that initiated the rotation (`keys add alice`, `keys remove
	// bob`). Surfaced in recovery diagnostics so users understand
	// what was interrupted.
	Operation string `yaml:"operation"`
}

// Validate rejects sentinels that clearly can't drive a
// meaningful recovery — missing fields, version inversion,
// obviously-wrong absolute paths.
func (s *ResealSentinel) Validate() error {
	if s.FromVersion < 1 {
		return fmt.Errorf("reseal sentinel: invalid from_version %d", s.FromVersion)
	}
	if s.ToVersion <= s.FromVersion {
		return fmt.Errorf("reseal sentinel: to_version %d must exceed from_version %d",
			s.ToVersion, s.FromVersion)
	}
	if !filepath.IsAbs(s.RepoRoot) {
		return fmt.Errorf("reseal sentinel: repo_root %q must be absolute", s.RepoRoot)
	}
	if len(s.NewRecipients) == 0 {
		return errors.New("reseal sentinel: no new recipients")
	}
	if s.Operation == "" {
		return errors.New("reseal sentinel: empty operation label")
	}
	return nil
}

// WriteResealSentinel persists s under the per-repo XDG state
// directory keyed by repoID. Writes atomically (tmp + rename);
// callers can assume that after a successful return the file is
// durable or doesn't exist at all, never partial.
func WriteResealSentinel(repoID string, s *ResealSentinel) error {
	if err := s.Validate(); err != nil {
		return err
	}
	state, err := NewLocalState(repoID)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(state.root, stateDirPerm); err != nil {
		return fmt.Errorf("reseal sentinel: prepare state dir: %w", err)
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("reseal sentinel: marshal: %w", err)
	}
	path := filepath.Join(state.root, resealSentinelFile)
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, data, stateFilePerm); err != nil {
		return fmt.Errorf("reseal sentinel: write: %w", err)
	}
	return os.Rename(tmp, path)
}

// ReadResealSentinel loads the sentinel for repoID, or returns
// os.ErrNotExist if none exists (the normal case). Callers
// distinguish "no rotation in flight" from "sentinel present but
// malformed" via errors.Is.
func ReadResealSentinel(repoID string) (*ResealSentinel, error) {
	state, err := NewLocalState(repoID)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(state.root, resealSentinelFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s ResealSentinel
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("reseal sentinel: parse: %w", err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteResealSentinel removes the sentinel for repoID. Idempotent:
// a missing file is not an error.
func DeleteResealSentinel(repoID string) error {
	state, err := NewLocalState(repoID)
	if err != nil {
		return err
	}
	path := filepath.Join(state.root, resealSentinelFile)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reseal sentinel: delete: %w", err)
	}
	return nil
}

// NewRecipientList parses NewRecipients into concrete Recipient
// objects, sorted by name (matching the deterministic order used
// by Keyring.Recipients so age stanza ordering stays stable).
func (s *ResealSentinel) NewRecipientList() ([]Recipient, error) {
	names := make([]string, 0, len(s.NewRecipients))
	for n := range s.NewRecipients {
		names = append(names, n)
	}
	sortStrings(names)
	out := make([]Recipient, 0, len(names))
	for _, n := range names {
		r, err := ParseRecipient(s.NewRecipients[n])
		if err != nil {
			return nil, fmt.Errorf("reseal sentinel: %s: %w", n, err)
		}
		out = append(out, r)
	}
	return out, nil
}
