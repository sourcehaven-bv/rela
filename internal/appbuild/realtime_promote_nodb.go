//go:build !postgres && !sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// txChannelsFor returns nil: the filesystem and memory tiers have no
// transaction the channel store could join.
//
// Nil: always returned.
func txChannelsFor(store.Store, channels.Store) joinChannels { return nil }
