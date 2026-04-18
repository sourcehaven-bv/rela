package metamodel

import "github.com/Sourcehaven-BV/rela/internal/entity"

// ScriptContext provides everything a script executor needs to execute.
// This interface is defined here (instead of the script package) to allow
// workspace to implement it without importing script, avoiding import cycles.
//
// The GetWorkspace() method returns an interface{} that script consumers
// type-assert to lua.Services. Returning interface{} avoids a workspace→lua
// import and lets non-lua callers implement ScriptContext too.
type ScriptContext interface {
	// GetWorkspace returns the services bundle for Lua callbacks.
	// The returned value must be a lua.Services value.
	GetWorkspace() interface{}
	// GetMeta returns the current metamodel.
	GetMeta() *Metamodel
	// GetProjectRoot returns the absolute project path.
	GetProjectRoot() string
	// GetEntity returns the triggering entity (may be nil).
	GetEntity() *entity.Entity
	// GetOldEntity returns the previous entity state (may be nil).
	GetOldEntity() *entity.Entity
}
