//go:build !sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// layerStoreConfig returns the filesystem loader unchanged: on this build no
// store carries operator config, so there is no second layer to fall back to.
//
// Split by build tag rather than by a runtime type assertion so a build whose
// store cannot carry config does not link the code that would layer it.
func layerStoreConfig(disk config.Loader, _ store.Store) config.Loader {
	return disk
}
