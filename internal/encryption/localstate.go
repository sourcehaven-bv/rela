package encryption

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// LocalState owns the per-machine, per-repo state that MUST NOT be
// synced to any untrusted storage. Specifically: the highest
// encryption version this machine has observed for a given repo.
// Keeping this outside the repo directory is what makes rollback
// detection work — an adversary who can replace files inside
// <root>/ cannot simultaneously roll back whatever we remember in
// the user's config tree.
//
// A missing file is TOFU: the first read returns (0, nil) and the
// caller is expected to accept whatever version it sees on disk
// and StoreVersion it back. Subsequent reads then enforce
// "observed ≥ stored" — downgrades are refused.
//
// The state is scoped by the userstate.FSService the caller passes
// in: different rela projects on the same machine get different
// per-repo services, different state files, and therefore
// independent version tracking.
type LocalState struct {
	svc userstate.FSService
}

// NewLocalState wraps an FSService that is already scoped to the
// current repo. Callers build the service via userstate.Open (or
// OpenWithKeyringID for encrypted repos) at factory time and thread
// it through; this constructor never resolves a path of its own.
func NewLocalState(svc userstate.FSService) (*LocalState, error) {
	if svc == nil {
		return nil, errors.New("encryption: NewLocalState: nil service")
	}
	return &LocalState{svc: svc}, nil
}

// versionKey names the user-state key holding the last-seen
// version. Plaintext integer. The file lives outside the synced
// tree, so the adversary we're defending against (cloud storage)
// can't see or tamper with it.
const versionKey = "last_seen_version"

// LoadVersion returns the highest version this machine has observed
// for this repo. Returns 0 when no state exists yet (TOFU).
//
// A corrupt file (non-integer content) is treated as
// ErrCorruptedLocalState rather than silently resetting to 0 — if
// the on-disk value is unparseable something is wrong and we'd
// rather surface it than accept an attacker-controlled reset.
func (s *LocalState) LoadVersion() (int, error) {
	data, err := s.svc.Get(context.Background(), versionKey)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("encryption: load last-seen version: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("%w: %s: %w", ErrCorruptedLocalState, versionKey, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("%w: negative version %d", ErrCorruptedLocalState, n)
	}
	return n, nil
}

// StoreVersion persists v as the new last-seen version for this
// repo. The caller is responsible for only advancing: passing a
// lower v than the stored value would silently weaken the rollback
// defense, and StoreVersion does NOT guard against that (callers
// already have the observed-vs-stored comparison in hand by the
// time they reach this point).
//
// Inter-process correctness is enforced by taking an advisory lock
// on the versionKey sidecar — two rela processes calling
// StoreVersion concurrently serialize rather than interleave. The
// atomic write inside Put handles single-process crash safety.
func (s *LocalState) StoreVersion(v int) error {
	unlock, err := s.svc.Lock(versionKey)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()

	if err := s.svc.Put(context.Background(), versionKey,
		[]byte(strconv.Itoa(v)+"\n")); err != nil {
		return fmt.Errorf("encryption: store last-seen version: %w", err)
	}
	return nil
}

// ErrCorruptedLocalState indicates the per-repo state file
// contained something other than a non-negative integer. Callers
// can surface this to the user as "delete the state file to TOFU
// again" — the file lives on the local machine, so recovery is a
// user decision, not a crypto one.
var ErrCorruptedLocalState = errors.New("encryption: corrupted local state")

// Service exposes the underlying FSService. Primarily used by
// helpers that need to place related files (reseal sentinel) in
// the same per-repo directory.
func (s *LocalState) Service() userstate.FSService { return s.svc }
