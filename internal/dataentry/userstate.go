package dataentry

import (
	"context"
	"encoding/json"
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
