package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A delete or rename a channel type blocks is reported as a conflict that
// names the entity and the type, and still wraps the typed error; any other
// error passes through unchanged.
func TestAnchorConflict(t *testing.T) {
	blocked := fmt.Errorf("delete: %w", &entitymanager.AnchorBlockedError{ChannelType: "incident-room"})
	err := anchorConflict("INC-1", blocked)
	for _, w := range []string{"conflict:", "INC-1", "incident-room"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q lacks %q", err, w)
		}
	}
	var typed *entitymanager.AnchorBlockedError
	if !errors.As(err, &typed) {
		t.Error("the typed error must stay in the chain")
	}

	raced := anchorConflict("INC-1",
		fmt.Errorf("delete: owned entities changed: %w", entitymanager.ErrConcurrentChange))
	for _, w := range []string{"conflict:", "INC-1", "retry"} {
		if !strings.Contains(raced.Error(), w) {
			t.Errorf("error %q lacks %q", raced, w)
		}
	}
	if !errors.Is(raced, entitymanager.ErrConcurrentChange) {
		t.Error("ErrConcurrentChange must stay in the chain")
	}

	// Only the guard's sentinel gets the retry hint, not every store conflict.
	storeConflict := fmt.Errorf("create: %w", store.ErrConflict)
	if got := anchorConflict("INC-1", storeConflict); got != storeConflict { //nolint:errorlint // identity is the assertion
		t.Errorf("anchorConflict(store conflict) = %v, want it unchanged", got)
	}

	other := errors.New("disk full")
	if got := anchorConflict("INC-1", other); got != other { //nolint:errorlint // identity is the assertion
		t.Errorf("anchorConflict(other) = %v, want it unchanged", got)
	}
}
