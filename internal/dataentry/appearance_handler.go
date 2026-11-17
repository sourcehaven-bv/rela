package dataentry

import (
	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// appearanceHandler owns the theme/settings/palette surface: the
// /_theme/logo CRUD (handlers_theme.go), the /_theme/export and
// /_theme/import package endpoints (handlers_theme_package.go), and the
// /_settings and /_palette CRUD (settings_handlers.go). Extracted from
// App (TKT-8AJ1PM, part of the TKT-R68TV8 decomposition arc).
//
// Collaborator rationale mirrors viewsHandler/writeHandler: fixed
// service handles by value (logo/palette/settings are self-synchronized
// services set once per wiring and never swapped by tests), and the schema
// snapshot as a closure so a live config reload propagates. It reads no
// entities: the settings page picks relation-default targets through the
// search endpoint (TKT-WJP6PBM). Every write here lands on a
// self-synchronized service, so there is no entitymanager.
type appearanceHandler struct {
	schema func() *Schema
	// logo owns the user-uploaded sidebar logo (persistence + served cache).
	logo *logoStore
	// palette owns the user palette override + resolved palette.
	palette *paletteService
	// settings owns the per-user default values.
	settings *settingsService
	// acl reads the App's ACL per call, as affordanceService does, because
	// tests swap it after construction. Saving the shared defaults is
	// decided by its mode.
	acl func() acl.ACL
}

// newAppearanceHandler wires the handler over the App's current
// collaborators. Called from NewApp and from the test rebind path after
// the logo/palette/settings services are set, so the
// fixed handles are the same instances the App holds.
func newAppearanceHandler(app *App) *appearanceHandler {
	return &appearanceHandler{
		schema:   app.State,
		logo:     app.logo,
		palette:  app.palette,
		settings: app.settings,
		acl:      func() acl.ACL { return app.acl },
	}
}
