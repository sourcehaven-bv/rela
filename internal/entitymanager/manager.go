// Package entitymanager owns the high-level entity lifecycle
// operations: create, update, delete, and the unified update-with-relations
// path used by the data-entry HTTP API. It coordinates the workspace
// transaction primitive, the automation engine (synchronous property
// hooks inside the transaction; side effects after commit), validation,
// and graph mutation.
//
// Today the package re-exposes operations that historically lived on
// workspace.Workspace. The intent is to migrate callers (cli, mcp, lua,
// dataentry) to this package over time so workspace can shrink to a
// pure storage / graph-state primitive.
package entitymanager

import (
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// Manager exposes entity lifecycle operations on top of a Workspace.
// It is safe for concurrent use insofar as the underlying workspace
// methods are: callers that need stricter ordering must serialize at a
// higher level (the data-entry server does this via App.writeMu around
// the HTTP handler).
type Manager struct {
	ws *workspace.Workspace
}

// New returns a Manager bound to the given workspace.
func New(ws *workspace.Workspace) *Manager {
	return &Manager{ws: ws}
}

// Workspace returns the underlying workspace. Provided for callers that
// still need direct workspace access during the migration; new code
// should prefer the typed methods on Manager.
func (m *Manager) Workspace() *workspace.Workspace {
	return m.ws
}
