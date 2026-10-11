//go:build postgres || sqlite

package appbuild_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// TestNew_AssemblesAVersionSweep is the guard for the build's real store.
//
// The backend sweep is found by concrete store type, and a type miss yields
// no sweep and no error: correct for a test double, but for the store the
// recipe actually opens it would mean create/update history silently stops.
// A compile-time assertion on the sweep's method set cannot see that; only
// building through the recipe can. On postgres this needs
// RELA_TEST_DATABASE_URL (see backendtest).
func TestNew_AssemblesAVersionSweep(t *testing.T) {
	svc, err := appbuild.Discover(writeMinimalProject(t), script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	require.True(t, appbuild.HasVersionSweep(svc),
		"the recipe's store got no version sweep; create and update versions "+
			"would never be captured, since only rename and delete are synchronous")
}
