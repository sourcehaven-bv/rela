package dataentry

import (
	"errors"
	"testing"

	synctypes "github.com/Sourcehaven-BV/rela/internal/sync"
)

// TestFilterVisibleManifest_SourceReadFaultFails pins that a failed read of the
// relation sources fails the whole manifest. A relation is gated on its
// source's type, so serving the entry without that type would gate it on
// nothing.
func TestFilterVisibleManifest_SourceReadFaultFails(t *testing.T) {
	boom := errors.New("store down")
	h := &syncHandler{store: &flakyStore{err: boom}}
	entries := []synctypes.ManifestEntry{{Kind: "r", IDA: "TKT-1"}}
	if _, err := h.filterVisibleManifest(t.Context(), entries); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the read fault", err)
	}
}
