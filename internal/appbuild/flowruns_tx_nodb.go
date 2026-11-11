//go:build !postgres && !sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// joinFlowRunsFor returns nil: the filesystem and memory tiers keep runs in
// a KV store that has no transaction in common with the entity store.
//
// Nil: always returned.
func joinFlowRunsFor(store.Store, flowrun.Store) joinFlowRuns { return nil }
