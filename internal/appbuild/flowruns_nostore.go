//go:build !postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// storeFlowRunsFor returns nil outside the postgres build: the fs, memory
// and desktop tiers keep runs in the cache directory's KV, and sqlite
// supplies its run store as a recipe override.
func storeFlowRunsFor(_ store.Store) flowrun.Store { return nil }
