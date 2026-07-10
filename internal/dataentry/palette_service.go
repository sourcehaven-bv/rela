package dataentry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// errInvalidPalette wraps a validation failure from [paletteService.Save] so
// the HTTP handler can map it to 400 (client error) while a persistence
// failure maps to 500. errors.Is against this sentinel is the seam.
var errInvalidPalette = errors.New("invalid palette")

// paletteService owns the theme palette: the user's raw override
// (`.rela/palette.yaml`, edited via the settings UI) AND the resolved palette
// the SPA/apps consume, which is a pure derivation of the project palette
// (`Cfg.Palette`) layered with the user override.
//
// It is self-synchronizing (its own RWMutex) and NOT part of the App-wide
// AppState snapshot. Because the resolved palette depends on `Cfg.Palette`,
// which is reload state still held in AppState, the config→palette dependency
// is made explicit here as [paletteService.Reresolve]: the reload path and the
// save path both hand the service the current project palette and it
// recomputes. When Cfg becomes its own provider (schema coherence-core), this
// service reads from that provider instead of being fed the value.
type paletteService struct {
	kv state.KV

	mu       sync.RWMutex
	user     *PaletteConfig   // raw user override; nil == "no user palette"
	resolved *ResolvedPalette // derived from (cfgPalette, user)
}

// newPaletteService builds the service over the given KV, loading the user
// palette and resolving it against the supplied project palette. A read error
// is surfaced (not masked) so a corrupt palette.yaml doesn't get silently
// overwritten by the next save (RR-OA4A).
func newPaletteService(kv state.KV, cfgPalette *PaletteConfig) (*paletteService, error) {
	s := &paletteService{kv: kv}
	user, err := s.load()
	if err != nil {
		return nil, err
	}
	s.user = user
	s.resolved = ResolvePalette(cfgPalette, user)
	return s, nil
}

// UserPalette returns the raw user override, or nil when none is saved.
func (s *paletteService) UserPalette() *PaletteConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.user
}

// Resolved returns the resolved palette the SPA and apps consume.
func (s *paletteService) Resolved() *ResolvedPalette {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resolved
}

// Save validates then persists the user palette and recomputes the resolved
// palette against the supplied project palette, publishing both atomically.
// The on-disk write happens first; the cache advances only on success.
func (s *paletteService) Save(ctx context.Context, cfgPalette, input *PaletteConfig) error {
	if err := dataentryconfig.ValidatePalette(input); err != nil {
		return fmt.Errorf("%w: %w", errInvalidPalette, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.persist(ctx, input); err != nil {
		return err
	}
	s.user = input
	s.resolved = ResolvePalette(cfgPalette, input)
	return nil
}

// Reresolve re-reads the user palette from disk and recomputes the resolved
// palette against the (possibly new) project palette. Called on config/meta
// reload. If the on-disk palette can't be read, the previous user palette is
// kept (better stale colors than a wipe) and the resolved palette is still
// recomputed against the new project palette.
func (s *paletteService) Reresolve(cfgPalette *PaletteConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, err := s.load()
	if err != nil {
		// Keep the previous user palette; still re-resolve against new cfg.
		s.resolved = ResolvePalette(cfgPalette, s.user)
		return err
	}
	s.user = user
	s.resolved = ResolvePalette(cfgPalette, user)
	return nil
}

// load reads .rela/palette.yaml. Returns (nil, nil) when the file does not
// exist (clean "no user palette" state). Returns a non-nil error if the file
// exists but can't be read/parsed — callers MUST surface this rather than
// silently falling back to defaults (RR-OA4A).
//
//nolint:nilnil // (nil, nil) is the documented "no user palette" signal
func (s *paletteService) load() (*PaletteConfig, error) {
	if s.kv == nil {
		return nil, nil
	}
	data, err := s.kv.Get(context.Background(), userPaletteFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", userPaletteFile, err)
	}
	var p PaletteConfig
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w (legacy `dark: auto` is no longer supported — remove the `dark` line or set it to `false` or an explicit object)", userPaletteFile, err)
	}
	return &p, nil
}

// persist writes the user palette to .rela/palette.yaml. Caller holds s.mu.
func (s *paletteService) persist(ctx context.Context, p *PaletteConfig) error {
	if s.kv == nil {
		return nil
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return s.kv.Put(ctx, userPaletteFile, data)
}
