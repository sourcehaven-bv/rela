//go:build !postgres && !sqlite

package appbuild

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestVersionTaggerForIsNilWithoutVersioning pins that a build with no
// database version history hands out no tagger, as an untyped nil the caller
// can check.
func TestVersionTaggerForIsNilWithoutVersioning(t *testing.T) {
	got, err := versionTaggerFor(nil, metamodel.DefaultMetamodel())
	if err != nil || got != nil {
		t.Fatalf("versionTaggerFor = %#v, %v; want nil, nil", got, err)
	}
}
