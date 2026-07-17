package dataentry

import (
	"context"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/state"
)

// settingsService owns the per-user default values (`.rela/user-defaults.yaml`)
// edited via the settings UI: the create-form defaults and relation defaults
// the SPA pre-fills.
//
// Self-synchronizing (its own RWMutex), extracted from the App-wide AppState
// snapshot. Unlike the palette/logo services, a bad user-defaults file is
// non-fatal — it's a UI convenience, not data the next save could destroy — so
// a read error degrades to "no defaults" rather than surfacing, matching the
// prior behavior.
type settingsService struct {
	kv state.KV

	mu       sync.RWMutex
	defaults *UserDefaults
}

// newSettingsService builds the service over the given KV and loads the
// persisted defaults (nil when absent/unreadable — non-fatal by design).
func newSettingsService(kv state.KV) *settingsService {
	s := &settingsService{kv: kv}
	s.defaults = s.load()
	return s
}

// UserDefaults returns the current defaults, or nil when none are saved.
func (s *settingsService) UserDefaults() *UserDefaults {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defaults
}

// Save persists the defaults and updates the cache atomically. The on-disk
// write happens first; the cache advances only on success.
func (s *settingsService) Save(ctx context.Context, ud *UserDefaults) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.persist(ctx, ud); err != nil {
		return err
	}
	s.defaults = ud
	return nil
}

// load reads .rela/user-defaults.yaml. Returns nil if the file doesn't exist or
// can't be parsed — user defaults are a non-critical UI convenience.
func (s *settingsService) load() *UserDefaults {
	if s.kv == nil {
		return nil
	}
	data, err := s.kv.Get(context.Background(), userDefaultsFile)
	if err != nil {
		return nil
	}
	var ud UserDefaults
	if err := yaml.Unmarshal(data, &ud); err != nil {
		return nil
	}
	return &ud
}

// persist writes the defaults to .rela/user-defaults.yaml. Caller holds s.mu.
func (s *settingsService) persist(ctx context.Context, ud *UserDefaults) error {
	if s.kv == nil {
		return nil
	}
	data, err := yaml.Marshal(ud)
	if err != nil {
		return err
	}
	return s.kv.Put(ctx, userDefaultsFile, data)
}
