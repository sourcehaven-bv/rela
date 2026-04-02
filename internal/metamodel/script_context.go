package metamodel

import "github.com/Sourcehaven-BV/rela/internal/model"

// ScriptContext provides everything a script executor needs to execute.
// This interface is defined here (instead of the script package) to allow
// workspace to implement it without importing script, avoiding import cycles.
//
// The GetWorkspace() method returns an interface{} which must satisfy
// lua.WorkspaceInterface. This avoids workspace needing to import lua
// just to declare the return type.
type ScriptContext interface {
	// GetWorkspace returns the workspace for Lua callbacks.
	// The returned value must satisfy lua.WorkspaceInterface.
	GetWorkspace() interface{}
	// GetMeta returns the current metamodel.
	GetMeta() *Metamodel
	// GetProjectRoot returns the absolute project path.
	GetProjectRoot() string
	// GetEntity returns the triggering entity (may be nil).
	GetEntity() *model.Entity
	// GetOldEntity returns the previous entity state (may be nil).
	GetOldEntity() *model.Entity
}
