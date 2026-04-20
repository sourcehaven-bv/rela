package userstate

import (
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// Service is the narrow contract consumed by components that only
// need to read and write user-scoped key/value state. It embeds
// state.KV so existing consumers (data-entry, scheduler) see no
// interface change; the concrete backend is what shifts.
type Service interface {
	state.KV
}

// FSService is the filesystem-backed superset used by components
// that need to resolve a concrete path — today that is the age
// identity reader in internal/encryption and the rela keys init
// CLI entry point.
//
// Path returns the absolute on-disk path for a given key; callers
// must not rely on the path being writable without going through
// Put, since future backends may synthesize paths lazily. Lock
// returns an unlocker for serializing compound operations on key —
// the file identified by Path(key + ".lock").
type FSService interface {
	Service
	// Root returns the absolute per-repo directory. Useful for
	// diagnostics ("where does my state live?") and for components
	// that want to build their own KV on top.
	Root() string
	// Path returns the absolute path of key under Root. Callers that
	// need to write through non-KV code (age identity writer, custom
	// formats) use this to discover the path; otherwise prefer Put.
	Path(key string) string
	// Lock acquires an exclusive advisory lock on a sidecar file
	// (Path(key) + ".lock"). Release by calling the returned
	// unlock function. Use for compound read-compare-write
	// operations that must serialize across processes.
	Lock(key string) (unlock func() error, err error)
}
