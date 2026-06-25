package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/state"
)

// userStateStore persists per-user UI state to the project's `.rela/` KV store:
// the sidebar logo, UI state (expanded groups, active list), user defaults, and
// the user palette override. Extracted from App (TKT-N26KLB M5.3): every method
// is a load/save pair over the KV store and touches nothing else, so they form
// a cohesive store with a single dependency.
//
// These are NOT the entity store — they're the per-user customization layer
// that rides alongside it (gitignored `.rela/` files). Writes here do not go
// through entitymanager; they are local UI preferences, not graph mutations.
type userStateStore struct {
	kv state.KV
}

// loadUIState reads .rela/ui-state.json and returns the persisted state.
// Returns an empty UIState if the file doesn't exist or can't be parsed.
func (s userStateStore) loadUIState(ctx context.Context) UIState {
	st := UIState{CollapsedGroups: make(map[string]bool)}
	if s.kv == nil {
		return st
	}
	data, err := s.kv.Get(ctx, uiStateFile)
	if err != nil {
		return st
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return UIState{CollapsedGroups: make(map[string]bool)}
	}
	if st.CollapsedGroups == nil {
		st.CollapsedGroups = make(map[string]bool)
	}
	return st
}

// saveUIState writes the UI state to .rela/ui-state.json.
func (s userStateStore) saveUIState(st UIState) error {
	if s.kv == nil {
		return nil
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return s.kv.Put(context.Background(), uiStateFile, data)
}

// loadUserDefaults reads .rela/user-defaults.yaml and returns the parsed defaults.
// Returns nil if the file doesn't exist or can't be parsed.
func (s userStateStore) loadUserDefaults() *UserDefaults {
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

// saveUserDefaults writes the user defaults to .rela/user-defaults.yaml.
func (s userStateStore) saveUserDefaults(ctx context.Context, ud *UserDefaults) error {
	if s.kv == nil {
		return nil
	}
	data, err := yaml.Marshal(ud)
	if err != nil {
		return err
	}
	return s.kv.Put(ctx, userDefaultsFile, data)
}

// loadUserPalette reads .rela/palette.yaml and returns the parsed
// palette. Returns (nil, nil) when the file does not exist (clean
// "no user palette" state — matches how ResolvePalette consumes a
// nil user palette pointer; a sentinel error or three-return shape
// would be more confusing for the only two callers). Returns a
// non-nil error if the file exists but cannot be read or parsed —
// callers MUST surface this instead of silently falling back to
// defaults, otherwise a subsequent save would silently overwrite
// the user's palette with framework defaults (RR-OA4A).
//
//nolint:nilnil // see comment above
func (s userStateStore) loadUserPalette() (*PaletteConfig, error) {
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

// saveUserPalette writes the user palette to .rela/palette.yaml.
func (s userStateStore) saveUserPalette(ctx context.Context, p *PaletteConfig) error {
	if s.kv == nil {
		return nil
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return s.kv.Put(ctx, userPaletteFile, data)
}

// loadUserLogo reads the persisted logo bytes and extension. Returns
// (nil, "", nil) when no logo is set. A sidecar file present without
// matching bytes (or vice versa) is treated as "no logo set" so a
// half-written state during a crash doesn't trip up the boot path.
//
// Returns a non-nil error only when the kv layer reports an unexpected
// failure (corrupt filesystem, permission denied) — callers should
// surface those instead of silently masking them.
//
// Concurrency: NOT safe to call in parallel with saveUserLogo or
// deleteUserLogo (the bytes/sidecar pair is read non-atomically). Boot
// path calls this before publishing the App; future reload paths must
// hold writeMu (i.e. invoke from inside mutateState).
//
//nolint:gocritic // unnamedResult: handler-style (bytes, ext, err) is clearer than naming for two callers
func (s userStateStore) loadUserLogo() ([]byte, string, error) {
	if s.kv == nil {
		return nil, "", nil
	}
	ctx := context.Background()

	logoBytes, err := s.kv.Get(ctx, userLogoFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("read %s: %w", userLogoFile, err)
	}

	extBytes, err := s.kv.Get(ctx, userLogoExtFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
			// Bytes present but sidecar missing — treat as not-set.
			// The user can re-upload to recover; we don't proactively
			// clean up so a future fix can recover the bytes.
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("read %s: %w", userLogoExtFile, err)
	}

	ext := string(extBytes)
	if _, ok := allowedLogoExts[ext]; !ok {
		// Sidecar contains an unknown extension — treat as not-set.
		// Same reasoning as the missing-sidecar case.
		return nil, "", nil
	}

	return logoBytes, ext, nil
}

// saveUserLogo persists the bytes and extension. Caller must hold
// writeMu (i.e. invoke from inside mutateState) so the bytes/sidecar
// pair cannot be observed half-written by a concurrent reload.
func (s userStateStore) saveUserLogo(ctx context.Context, bytes []byte, ext string) error {
	if s.kv == nil {
		return errors.New("kv not configured")
	}
	if _, ok := allowedLogoExts[ext]; !ok {
		return fmt.Errorf("invalid logo extension %q", ext)
	}
	if err := s.kv.Put(ctx, userLogoFile, bytes); err != nil {
		return fmt.Errorf("write %s: %w", userLogoFile, err)
	}
	if err := s.kv.Put(ctx, userLogoExtFile, []byte(ext)); err != nil {
		return fmt.Errorf("write %s: %w", userLogoExtFile, err)
	}
	return nil
}

// deleteUserLogo removes both the bytes file and the sidecar. Idempotent;
// missing files are not errors.
func (s userStateStore) deleteUserLogo(ctx context.Context) error {
	if s.kv == nil {
		return nil
	}
	if err := s.kv.Delete(ctx, userLogoFile); err != nil {
		return fmt.Errorf("delete %s: %w", userLogoFile, err)
	}
	if err := s.kv.Delete(ctx, userLogoExtFile); err != nil {
		return fmt.Errorf("delete %s: %w", userLogoExtFile, err)
	}
	return nil
}
