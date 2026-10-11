package dataentry

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// pilesHandler owns the piles routes (TKT-K3RJLH): a user's private, named
// collections of entity references.
//
// Extracted from App (the commentsHandler pattern) to keep App under its
// plimsoll method load line. The service is nil until [App.SetPiles] runs,
// and nil is the "feature absent" signal: the routes 404 and the bootstrap
// reports piles_available false.
type pilesHandler struct {
	svc *piles.Service
	// scripts is rela.piles for action scripts; set with svc.
	scripts ScriptPiles

	meta func() *metamodel.Metamodel
	cfg  func() *Config
	// resolver serves pile items through the row gate and the field
	// redactor, in one batch per request.
	resolver pileResolver
	// export is late-bound because tests rebuild app.export after
	// construction.
	export func() *exportHandler
}

// newPilesHandler builds the handler over app's collaborators. A handler is
// returned even with no service, so the routes answer a JSON 404.
func newPilesHandler(app *App) (*pilesHandler, error) {
	reader, err := visibility.NewPolicyReader(ctxRowGate{}, appRedactor(app), app.store, familiesOption(app))
	if err != nil {
		return nil, fmt.Errorf("dataentry: newPilesHandler: %w", err)
	}
	return &pilesHandler{
		meta:     app.Meta,
		cfg:      func() *Config { return app.State().Cfg },
		resolver: reader.Resolver(),
		export:   func() *exportHandler { return app.export },
	}, nil
}

// ScriptPiles is the rela.piles capability action scripts get: the Lua
// adapter over the same service, built at the wiring site.
type ScriptPiles interface {
	lua.PileReader
	lua.PileWriter
}

// SetPiles installs the piles service and its Lua adapter. Nil: rejected for
// both — a deployment without piles simply never calls this, so a nil here
// is a wiring bug.
func (a *App) SetPiles(svc *piles.Service, scripts ScriptPiles) error {
	if svc == nil || scripts == nil {
		return errors.New("dataentry: SetPiles requires a service and its script adapter")
	}
	a.piles.svc = svc
	a.piles.scripts = scripts
	return nil
}

// scriptPiles completes deps with rela.piles when piles are wired.
func (h *pilesHandler) scriptPiles(deps lua.WriteDeps) lua.WriteDeps {
	if h != nil && h.scripts != nil {
		deps.Piles = h.scripts
		deps.PileWriter = h.scripts
	}
	return deps
}

// available reports that piles are wired and the principal on ctx has an
// owner identity to keep them under.
func (h *pilesHandler) available(ctx context.Context) bool {
	if h == nil || h.svc == nil {
		return false
	}
	_, err := h.svc.Owner(ctx)
	return err == nil
}

// pilesWire is the `piles:` block as the bootstrap payloads serve it: nil
// stays null, and an absent list is an empty array, never null.
func pilesWire(pc *dataentryconfig.PilesConfig) *dataentryconfig.PilesConfig {
	if pc == nil {
		return nil
	}
	out := dataentryconfig.PilesConfig{Actions: []string{}, Export: []string{}}
	out.Actions = append(out.Actions, pc.Actions...)
	out.Export = append(out.Export, pc.Export...)
	return &out
}
