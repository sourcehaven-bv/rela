package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
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

	other := errors.New("disk full")
	if got := anchorConflict("INC-1", other); got != other { //nolint:errorlint // identity is the assertion
		t.Errorf("anchorConflict(other) = %v, want it unchanged", got)
	}
}
