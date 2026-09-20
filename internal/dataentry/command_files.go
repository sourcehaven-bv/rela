package dataentry

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// commandFileTTL bounds how long a minted download token stays usable after
// its run finishes.
//
// The clock starts at run COMPLETION, not at mint: the user clicks Download
// after the run reported the file, so expiring at mint plus a short window
// would race the very interaction the token exists to serve. A long-running
// script that emits a file early keeps that file downloadable for the whole
// run, then for this long afterwards.
const commandFileTTL = 30 * time.Minute

// commandFileEntry is one downloadable file produced by one command run.
//
// It holds the CommandConfig the run executed, not merely the permission name,
// because [authorizeCommand] decides over the whole config (Context and
// Permission both participate, and a future arm may read more). Storing the
// config means the download re-check calls the SAME function the exec boundary
// calls, with the same input, so the two cannot drift.
type commandFileEntry struct {
	// path is the absolute, symlink-resolved, containment-checked location on
	// disk. It is resolved once at mint time and never re-derived from client
	// input — the token is the only thing a caller supplies.
	path string
	// label is the display name, used as the download filename.
	label string
	// cmd is the command whose run produced this file. Re-authorized on every
	// download against the LIVE ACL.
	cmd CommandConfig
	// expires is the deadline, set when the run completes (see
	// [commandFileStore.release]). The zero value means the run is still in
	// flight and the token has not started aging.
	expires time.Time
}

// expired reports whether the token is past its deadline. A zero deadline is
// an in-flight run, which never expires — it is bounded by the run itself.
func (e commandFileEntry) expired(now time.Time) bool {
	return !e.expires.IsZero() && !now.Before(e.expires)
}

// commandFileStore maps opaque tokens to the files a command run produced.
//
// Tokens are capabilities, with two deliberate properties:
//
//   - The token names the file; the caller never does. A route that accepted a
//     caller-supplied path would be the arbitrary-read twin of the OS launcher
//     this replaced (TKT-93FUCV). Command output has no store key — scripts
//     write arbitrary paths — so containment at mint time plus an unguessable
//     handle is what stands in for one.
//   - Holding a token is not authorization. [commandFileStore.lookup] returns
//     the entry; the handler re-runs authorizeCommand against the current ACL
//     before streaming a byte. A token therefore stops working the moment the
//     grant that produced it is revoked (a --read-only restart, a policy edit),
//     rather than outliving it.
//
// Entries are in-memory and per-process. A restart drops every token, which is
// correct: the run that minted them is gone too.
type commandFileStore struct {
	mu sync.Mutex
	// byToken is the lookup index.
	byToken map[string]commandFileEntry
	// byRun groups tokens per exec id so a finished run can drop its own
	// entries without scanning the whole table.
	byRun map[string][]string
	// now is the clock, injectable for tests.
	now func() time.Time
}

func newCommandFileStore() *commandFileStore {
	return &commandFileStore{
		byToken: make(map[string]commandFileEntry),
		byRun:   make(map[string][]string),
		now:     time.Now,
	}
}

// mint registers a file under a fresh opaque token and returns it.
//
// The caller must have already resolved path through [containedProjectPath];
// mint does no containment check of its own, because it has no project root to
// check against. The returned token is 256 bits of crypto/rand — unguessable,
// so enumeration is not an attack path.
func (s *commandFileStore) mint(execID, path, label string, cmd CommandConfig) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	defer s.mu.Unlock()
	// Opportunistic sweep: the table is only ever touched by these three
	// methods, so there is no background goroutine to drain it. Minting is
	// the rare operation (one per emitted file), which makes it the cheap
	// place to pay for cleanup.
	s.sweepLocked()

	s.byToken[token] = commandFileEntry{
		path:  path,
		label: label,
		cmd:   cmd,
	}
	s.byRun[execID] = append(s.byRun[execID], token)
	return token, nil
}

// sweepLocked drops expired entries. Caller holds s.mu.
func (s *commandFileStore) sweepLocked() {
	now := s.now()
	for execID, tokens := range s.byRun {
		live := tokens[:0]
		for _, token := range tokens {
			if entry, ok := s.byToken[token]; ok && !entry.expired(now) {
				live = append(live, token)
				continue
			}
			delete(s.byToken, token)
		}
		if len(live) == 0 {
			delete(s.byRun, execID)
			continue
		}
		s.byRun[execID] = live
	}
}

// lookup returns the entry for a token, or false when the token is unknown or
// expired. An expired token is deleted on the way out.
//
// Returning the entry is NOT a grant: the caller must re-authorize the entry's
// command before serving its bytes.
func (s *commandFileStore) lookup(token string) (commandFileEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.byToken[token]
	if !ok {
		return commandFileEntry{}, false
	}
	if entry.expired(s.now()) {
		delete(s.byToken, token)
		return commandFileEntry{}, false
	}
	return entry, true
}

// release starts the expiry clock on every token a run minted. Called when the
// run finishes.
//
// It does NOT delete them: the user clicks Download after the run reports the
// file, so dropping the tokens at completion would break every button the run
// just rendered. [commandFileTTL] after this point they stop resolving, and
// the next mint sweeps them out of the table.
func (s *commandFileStore) release(execID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deadline := s.now().Add(commandFileTTL)
	for _, token := range s.byRun[execID] {
		entry, ok := s.byToken[token]
		if !ok {
			continue
		}
		entry.expires = deadline
		s.byToken[token] = entry
	}
}
