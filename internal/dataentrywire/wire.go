// Package dataentrywire connects a data-entry app to the services it was
// built from: the parts every surface serving the SPA needs, whatever else it
// adds.
//
// It exists because those parts are injected after [dataentry.NewApp], by
// setters, and a surface that forgets one gets no error. The desktop used to
// call none of them, so a list's `condition:` was "not compiled", worlds and
// next actions were missing, and nothing said why. One function called by
// every surface keeps them in step.
//
// It is a separate package because dataentry cannot import appbuild
// (dataentry's tests import appbuild, so the import would close a cycle), and
// this is where the two meet.
package dataentrywire

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
)

// Services wires app to the services in svc: CalDAV aliases, comments,
// worlds and their relation scopes, next-action user state, and the
// predicate compilers behind next-action sources, list and kanban
// `condition:`, and `query_scopes:`.
//
// Every failure is returned: a missing compiler leaves conditions unevaluated
// and scopes unapplied, which shows rows the operator excluded, and nothing
// on screen would say so. Identity, security and transport settings differ
// per surface and stay with the caller.
func Services(app *dataentry.App, svc *appbuild.Services) error {
	app.SetCalDAVAliases(svc.CalDAVAliases())
	app.SetComments(svc.Comments())

	// Selecting a world and resolving its links are wired together: a
	// surface that can select a world but not resolve its links renders every
	// page with no relations, which reads as a data problem.
	app.SetWorlds(appbuild.CompiledWorlds(svc))
	if err := dataentry.SetWorldNeighbors(app, svc.Store(), appbuild.RelationScopes(svc)); err != nil {
		return fmt.Errorf("wire world-scoped relations: %w", err)
	}

	if err := app.SetUserState(svc.UserState()); err != nil {
		return fmt.Errorf("wire next-action state: %w", err)
	}
	if err := app.SetNextActionMatchers(appbuild.NextActionMatchers); err != nil {
		return fmt.Errorf("wire next-action matchers: %w", err)
	}
	if err := app.SetViewConditions(dataentry.AdaptViewConditions(appbuild.ViewConditions)); err != nil {
		return fmt.Errorf("wire view conditions: %w", err)
	}
	if err := app.SetQueryScopeResolver(dataentry.AdaptQueryScopes(appbuild.QueryScopes)); err != nil {
		return fmt.Errorf("wire query scopes: %w", err)
	}
	return nil
}
