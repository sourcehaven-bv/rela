package peerkey

import (
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// ReloadInterval is how long a [Source] serves a keyring before it reads the
// key list again. It bounds how long a removed key keeps working.
const ReloadInterval = 10 * time.Second

// staleFactor bounds how long a keyring is served while a reload is stuck:
// past staleFactor intervals every key is refused with [ErrStale].
const staleFactor = 3

// ErrStale reports that the key list could not be reread in time, for
// example because the file read hangs on a stuck mount. Every key is refused
// until a reload completes, so a revoked key cannot outlive a hung read.
var ErrStale = errors.New("peerkey: the key list could not be reread in time")

// Loader returns the key list, peer name to stored digest. No list is an
// empty map and a nil error.
type Loader func() (map[string]string, error)

// SecretsLoader reads the `peer_key.*` entries of the project whose .rela
// directory is relaDir, from the systemd credential or `.rela/secrets.yaml`
// (see the secrets package). A project without a secrets source lists no key.
func SecretsLoader(relaDir string) Loader {
	return func() (map[string]string, error) {
		m, err := secrets.PeerKeyDigests(relaDir)
		if errors.Is(err, secrets.ErrNotFound) {
			return nil, nil
		}
		return m, err
	}
}

// Source verifies peer keys against a key list it re-reads at most every
// interval, so adding or removing a key needs no restart.
//
// The published keyring is an [atomic.Pointer]; a mutex serializes reloads.
// A reload is single-flight: the request that finds the keyring stale and
// takes the mutex reads the list, and every other request meanwhile uses the
// keyring already published, so no request waits on another's file read.
//
// A reload that hangs blocks no request, but after staleFactor intervals
// every key is refused with [ErrStale] until it completes.
//
// A list that cannot be read or parsed publishes an empty keyring: every
// peer key is refused until the list is fixed. Keeping the last good keyring
// instead would keep a key working after an edit meant to revoke it. The
// error is logged once per reload, naming the entry, never a value.
type Source struct {
	load     Loader
	now      func() time.Time
	interval time.Duration
	log      *slog.Logger

	cur    atomic.Pointer[snapshot]
	reload sync.Mutex
}

// snapshot is one published keyring and when it was read.
type snapshot struct {
	ring   *Keyring
	loaded time.Time
}

// NewSource reads the key list once and returns a Source over it.
//
// Nil: load and now are rejected, since a Source without them could verify
// nothing or never reload; log is accepted and means slog.Default(). A
// non-positive interval is rejected.
func NewSource(load Loader, interval time.Duration, now func() time.Time, log *slog.Logger) (*Source, error) {
	switch {
	case load == nil:
		return nil, errors.New("peerkey: NewSource: nil loader")
	case now == nil:
		return nil, errors.New("peerkey: NewSource: nil clock")
	case interval <= 0:
		return nil, errors.New("peerkey: NewSource: interval must be positive")
	}
	if log == nil {
		log = slog.Default()
	}
	s := &Source{load: load, now: now, interval: interval, log: log}
	s.refresh()
	return s, nil
}

// IsPeerKey reports whether credential is presented as a peer key; see the
// package function [IsPeerKey].
func (s *Source) IsPeerKey(credential string) bool { return IsPeerKey(credential) }

// Verify returns the peer name credential authenticates, reloading the key
// list first when it is older than the interval. The errors are those of
// [Keyring.Verify], and [ErrStale] while a reload is stuck.
func (s *Source) Verify(credential string) (string, error) {
	snap := s.cur.Load()
	if age := s.now().Sub(snap.loaded); age >= s.interval {
		if !s.reload.TryLock() {
			// Another request is reloading. Serve its predecessor, unless
			// that reload has hung for too long.
			if age >= staleFactor*s.interval {
				return "", ErrStale
			}
			return snap.ring.Verify(credential)
		}
		// Re-check under the lock: another request may have reloaded
		// between the Load above and the TryLock.
		if s.now().Sub(s.cur.Load().loaded) >= s.interval {
			s.refreshLocked()
		}
		s.reload.Unlock()
		snap = s.cur.Load()
	}
	return snap.ring.Verify(credential)
}

// refresh reloads under the mutex.
func (s *Source) refresh() {
	s.reload.Lock()
	defer s.reload.Unlock()
	s.refreshLocked()
}

// refreshLocked reads the list and publishes its keyring, or an empty one on
// an error. The caller holds s.reload.
func (s *Source) refreshLocked() {
	ring, err := s.read()
	if err != nil {
		s.log.Error("peerkey: the peer key list cannot be used; every peer key is refused until it is fixed",
			"err", err)
		ring = &Keyring{}
	}
	s.cur.Store(&snapshot{ring: ring, loaded: s.now()})
}

// read loads the list and builds its keyring.
func (s *Source) read() (*Keyring, error) {
	entries, err := s.load()
	if err != nil {
		return nil, err
	}
	return NewKeyring(entries)
}
