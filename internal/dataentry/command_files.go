package dataentry

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

// errNoCommandFileStore is returned by mint on a handler wired without a file
// store. Surfaced as a logged error and a file message with no download link,
// never as a failed command run.
var errNoCommandFileStore = errors.New("command file store not configured")

// commandFileTTL bounds how long a minted download token stays usable after
// its run finishes.
//
// The clock starts at run COMPLETION, not at mint: the user clicks Download
// after the run reported the file, so expiring at mint plus a short window
// would race the very interaction the token exists to serve. A long-running
// script that emits a file early keeps that file downloadable for the whole
// run, then for this long afterwards.
const commandFileTTL = 30 * time.Minute

// commandFileMaxLifetime is a backstop deadline stamped at mint time.
//
// The TTL above only starts on [commandFileStore.release], so an entry whose
// release never lands would otherwise be unreclaimable — `expired` would answer
// false forever and the sweep could never drop it. Every caller does release
// today (it is deferred before the first mint), which makes this belt and
// braces rather than a live fix; the point is that the table has a reclamation
// path that does not depend on anyone remembering.
//
// Generous on purpose: it must not cut short a legitimately long run, which is
// the property the release-triggered TTL exists to protect. A run still going
// after this long has a bigger problem than a dead download link.
const commandFileMaxLifetime = 12 * time.Hour

// containedPath is a filesystem path that has been proven to live inside the
// project root. It exists so that [commandFileStore.mint] cannot be handed a
// raw, unchecked path: a plain string parameter would document the
// precondition, but every string in the package would satisfy it, and the one
// call site that forgot would compile.
//
// The only constructor is [containProjectPath], so the type is a witness that
// the check ran rather than a claim that it did.
type containedPath struct {
	// abs is the absolute, symlink-resolved location.
	abs string
}

// containProjectPath is the sole way to obtain a [containedPath]. It is a thin
// wrapper over containedProjectPath, which stays string-returning for its other
// caller (resolveConflictPath in api_v1.go).
func containProjectPath(projectRoot, filePath string) (containedPath, error) {
	resolved, err := containedProjectPath(projectRoot, filePath)
	if err != nil {
		return containedPath{}, err
	}
	return containedPath{abs: resolved}, nil
}

// newRunKey returns an unguessable key identifying one command run, used to
// group that run's download tokens.
//
// It is NOT the run's `exec_id`. That one is client-supplied so the browser can
// address /api/command-cancel/, which means a caller can name another run's id;
// keying the token table on it would let one principal start the expiry clock
// on another's in-flight downloads. Since nothing outside the server needs to
// name a token group, the key is never accepted from a request.
//
// Falls back to a timestamp if the entropy source fails: a predictable group
// key only risks the early-expiry nuisance above, so degrading here is better
// than failing a command run that is otherwise fine.
func newRunKey() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// commandFileEntry is one downloadable file produced by one command run.
//
// It holds the CommandConfig the run executed, not merely the permission name,
// because [authorizeCommand] decides over the whole config (Context and
// Permission both participate, and a future arm may read more). Storing the
// config means the download re-check calls the SAME function the exec boundary
// calls, with the same input, so the two cannot drift.
type commandFileEntry struct {
	// path is the containment-checked location on disk. Resolved once at mint
	// time and never re-derived from client input — the token is the only thing
	// a caller supplies. Typed rather than a string so it cannot be populated
	// from an unchecked value; see [containedPath].
	path containedPath
	// label is the display name, used as the download filename.
	label string
	// cmd is the command whose run produced this file. Re-authorized on every
	// download against the LIVE ACL.
	cmd CommandConfig
	// expires is the deadline. Set to [commandFileMaxLifetime] at mint, then
	// SHORTENED to now+[commandFileTTL] when the run completes (see
	// [commandFileStore.release]). Never zero, so every entry is reclaimable
	// even if its release is missed.
	expires time.Time
}

// expired reports whether the token is past its deadline.
func (e commandFileEntry) expired(now time.Time) bool {
	return !now.Before(e.expires)
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
	// byRun indexes tokens by run key (see [newRunKey], NOT the client's
	// exec_id). It exists so [commandFileStore.release] can find one run's
	// tokens directly; the sweep does not use it as a shortcut.
	byRun map[string][]string
	// now is the clock, injectable for tests.
	now func() time.Time
}

// A nil *commandFileStore is USABLE and inert: mint declines to issue a token,
// lookup finds nothing, release does nothing.
//
// commandHandler is assembled by struct literal at two sites (app.go and the
// test helper), so the repo's "constructors reject nil required fields" rule
// has no constructor to live in here. A third site that forgets `files` would
// otherwise panic on a nil map deref inside an SSE handler — after the 200 and
// after the script already ran, which is the worst possible place to discover
// a wiring bug. Degrading to "no download buttons" makes that mistake visible
// without taking the run down with it.
//
// This is deliberately NOT the shape used for aclImpl, where nil means DENY:
// an authorization guard must fail closed, while a missing download table can
// only fail to offer a capability.

func newCommandFileStore() *commandFileStore {
	return &commandFileStore{
		byToken: make(map[string]commandFileEntry),
		byRun:   make(map[string][]string),
		now:     time.Now,
	}
}

// mint registers a file under a fresh opaque token and returns it.
//
// path is a [containedPath], so the containment check is enforced by the type
// rather than by this comment — mint has no project root of its own to check
// against. The returned token is 256 bits of crypto/rand: unguessable, so
// enumeration is not an attack path.
func (s *commandFileStore) mint(
	runKey string, path containedPath, label string, cmd CommandConfig,
) (string, error) {
	if s == nil {
		return "", errNoCommandFileStore
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	defer s.mu.Unlock()
	// Opportunistic sweep: nothing drains this table in the background, so
	// cleanup has to ride on a call. Mint is the right one — it runs once per
	// emitted file, where lookup runs once per download. (lookup also drops
	// the single entry it finds expired, but it never sees the rest.)
	s.sweepLocked()

	s.byToken[token] = commandFileEntry{
		path:    path,
		label:   label,
		cmd:     cmd,
		expires: s.now().Add(commandFileMaxLifetime),
	}
	s.byRun[runKey] = append(s.byRun[runKey], token)
	return token, nil
}

// sweepLocked drops expired entries. Caller holds s.mu.
//
// Cost is O(tokens) across every run, paid on mint. That is affordable only
// because mint is rare and the table holds one entry per file a command
// emitted; if either stops being true, this needs a real eviction structure
// rather than a full pass.
//
// `live := tokens[:0]` filters in place, reusing the existing backing array.
// Safe here because the write index never overtakes the read index and no
// slice header escapes the struct — do NOT hand a byRun slice to a caller
// without copying it first.
func (s *commandFileStore) sweepLocked() {
	now := s.now()
	for runKey, tokens := range s.byRun {
		live := tokens[:0]
		for _, token := range tokens {
			if entry, ok := s.byToken[token]; ok && !entry.expired(now) {
				live = append(live, token)
				continue
			}
			delete(s.byToken, token)
		}
		if len(live) == 0 {
			delete(s.byRun, runKey)
			continue
		}
		s.byRun[runKey] = live
	}
}

// lookup returns the entry for a token, or false when the token is unknown or
// expired. An expired token is deleted on the way out.
//
// Returning the entry is NOT a grant: the caller must re-authorize the entry's
// command before serving its bytes.
func (s *commandFileStore) lookup(token string) (commandFileEntry, bool) {
	if s == nil {
		return commandFileEntry{}, false
	}
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

// release brings every token a run minted down to the short post-run TTL.
// Called when the run finishes.
//
// It does NOT delete them: the user clicks Download after the run reports the
// file, so dropping the tokens at completion would break every button the run
// just rendered. [commandFileTTL] after this point they stop resolving, and
// the next mint sweeps them out of the table.
//
// The deadline only ever moves EARLIER. An entry already inside the window
// (a run somehow outliving [commandFileMaxLifetime]) keeps its shorter
// deadline rather than being granted a fresh extension by finishing.
func (s *commandFileStore) release(runKey string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	deadline := s.now().Add(commandFileTTL)
	for _, token := range s.byRun[runKey] {
		entry, ok := s.byToken[token]
		if !ok {
			continue
		}
		if deadline.Before(entry.expires) {
			entry.expires = deadline
			s.byToken[token] = entry
		}
	}
}
