package dataentry

import (
	"context"
	"encoding/json"

	"github.com/Sourcehaven-BV/rela/internal/state"
)

// userStateStore persists the SPA's UI state (expanded groups, active list) to
// `.rela/ui-state.json`. Originally a grab-bag of per-user customization
// (logo, palette, defaults, UI state); the logo/palette/defaults each moved
// into their own self-synchronized service, leaving this as the UI-state store.
//
// This is NOT the entity store — it's the per-user customization layer that
// rides alongside it (gitignored `.rela/` files). Writes here do not go through
// entitymanager; they are local UI preferences, not graph mutations.
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
