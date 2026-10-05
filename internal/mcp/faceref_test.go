package mcp

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestWholeEntityRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ref     string
		refused bool
	}{
		{"POL-001", false},
		{"POL-001@adopted", true},
		{"not an id", false},
	}
	for _, tc := range tests {
		t.Run(tc.ref, func(t *testing.T) {
			t.Parallel()
			if got := wholeEntityRef(tc.ref) != nil; got != tc.refused {
				t.Errorf("wholeEntityRef(%q) refused = %v, want %v", tc.ref, got, tc.refused)
			}
		})
	}
}

// A miss and a hidden entity read the same; any other failure must not read
// as a miss.
func TestEntityReadFailed(t *testing.T) {
	t.Parallel()
	miss := resultText(t, entityReadFailed("entity", "POL-001", store.ErrNotFound))
	if miss != "entity not found: POL-001" {
		t.Errorf("miss = %q", miss)
	}
	outage := resultText(t, entityReadFailed("entity", "POL-001", errors.New("world gone")))
	if strings.Contains(outage, "not found") {
		t.Errorf("an outage reads as a miss: %q", outage)
	}
}
