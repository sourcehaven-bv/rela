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
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
)

// Services wires app to the services in svc: CalDAV aliases, comments, piles, leases,
// the element range read behind /api/v1/_reconcile,
// the realtime.yaml holder, background `apply: auto` llm actions,
// the worlds' relation scopes, version tags, the Lua host config, next-action user state, and the
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
	if err := wireOptionalStores(app, svc); err != nil {
		return err
	}

	// The remote routes list every remote in remotes.yaml; a project
	// without one lists none.
	rs, rsErr := newRemotes(svc, slog.Default())
	if rsErr != nil {
		return fmt.Errorf("wire remotes: %w", rsErr)
	}
	if err := dataentry.SetRemotes(app, rs); err != nil {
		return fmt.Errorf("wire remotes: %w", err)
	}

	// The realtime holder is absent only for a Services built from
	// collaborators; realtime.yaml is then not watched.
	if r := appbuild.Realtime(svc); r != nil {
		if err := dataentry.SetRealtime(app, r); err != nil {
			return fmt.Errorf("wire realtime config: %w", err)
		}
	}
	if err := wireChannels(app, svc); err != nil {
		return err
	}

	// The worlds themselves are a NewApp argument; this resolves their links.
	// A surface that can select a world but not resolve its links renders
	// every page with no relations, which reads as a data problem.
	if err := dataentry.SetWorldNeighbors(app, svc.Store(), appbuild.RelationScopes(svc)); err != nil {
		return fmt.Errorf("wire world-scoped relations: %w", err)
	}

	// Version tags for action scripts (TKT-VO6VG9).
	dataentry.SetVersionTags(app, appbuild.ScriptVersionTags(svc), appbuild.VersionTagReader(svc))
	dataentry.SetOAuthTokens(app, appbuild.ScriptOAuth(svc))
	// The host config and seams of the services, so action, document and
	// webhook scripts see tenant mode's withheld secrets and ai.yaml rather
	// than the shared .rela directory.
	luaHost, luaSeams := appbuild.LuaHost(svc)
	dataentry.SetLuaHost(app, luaHost, luaSeams)

	// rela.attach_render for schedules and background automations
	// (TKT-1TS55R); action scripts get it from the app itself.
	renderer, err := dataentry.AttachRenderer(app)
	if err != nil {
		return fmt.Errorf("wire attach_render: %w", err)
	}
	if err = appbuild.SetAttachRenderer(svc, renderer); err != nil {
		return fmt.Errorf("wire attach_render: %w", err)
	}
	// rela.attach_file for the same scripts (TKT-36DHKH8).
	filer, err := dataentry.AttachFiler(app)
	if err != nil {
		return fmt.Errorf("wire attach_file: %w", err)
	}
	if err = appbuild.SetAttachFiler(svc, filer); err != nil {
		return fmt.Errorf("wire attach_file: %w", err)
	}

	if llmErr := llmActions(app, svc); llmErr != nil {
		return llmErr
	}

	// The reconcile endpoint (TKT-VP02WHG). A build failure is returned, not
	// degraded: an ungated element read would sum rows the caller may not read.
	elements, elErr := appbuild.Elements(svc)
	if elErr != nil {
		return fmt.Errorf("wire reconcile: %w", elErr)
	}
	if err := dataentry.SetReconcile(app, elements); err != nil {
		return fmt.Errorf("wire reconcile: %w", err)
	}

	if err := app.SetNextActions(svc.UserState(), appbuild.NextActionMatchers); err != nil {
		return fmt.Errorf("wire next actions: %w", err)
	}
	if err := app.SetViewConditions(dataentry.AdaptViewConditions(appbuild.ViewConditions)); err != nil {
		return fmt.Errorf("wire view conditions: %w", err)
	}
	if err := app.SetQueryScopeResolver(dataentry.AdaptQueryScopes(appbuild.QueryScopes)); err != nil {
		return fmt.Errorf("wire query scopes: %w", err)
	}
	return nil
}

// wireChannels sets the channel service, the channel file runner and the
// inbox grid on app. It does nothing when the project has no channel service.
func wireChannels(app *dataentry.App, svc *appbuild.Services) error {
	// Without a channel service the channel routes answer the entity 404
	// and the OpenAPI document has no channel paths.
	c := appbuild.ChannelService(svc)
	if c == nil {
		return nil
	}
	if err := dataentry.SetChannels(app, c); err != nil {
		return fmt.Errorf("wire channels: %w", err)
	}
	// Channel file scans use the runner the app checked; without one a
	// configured scan refuses the file.
	if r := dataentry.AttachmentRunner(app); r != nil {
		if err := appbuild.SetChannelFileRunner(svc, r); err != nil {
			return fmt.Errorf("wire channel files: %w", err)
		}
	}
	grid := dataentry.LiveInboxGrid
	if appbuild.ChannelsScan(svc) {
		grid = dataentry.LiveInboxGridScan
	}
	if err := dataentry.SetInboxGrid(app, grid); err != nil {
		return fmt.Errorf("wire inbox grid: %w", err)
	}
	return nil
}

// llmActions wires `llm:` actions with `apply: auto` (TKT-WQ925LL): the
// route queues runs where this process runs background jobs, and the job and
// flow steps run them through the app.
func llmActions(app *dataentry.App, svc *appbuild.Services) error {
	if q := appbuild.LLMActionQueue(svc); q != nil {
		if err := dataentry.SetLLMActionQueue(app, q); err != nil {
			return fmt.Errorf("wire llm actions: %w", err)
		}
	}
	llm, err := dataentry.NewLLMActionRunner(app)
	if err != nil {
		return fmt.Errorf("wire llm actions: %w", err)
	}
	if err = appbuild.SetLLMActionRunner(svc, llm); err != nil {
		return fmt.Errorf("wire llm actions: %w", err)
	}
	return nil
}

// wireOptionalStores wires the stores a backend or schema may not offer.
// Each one that is absent leaves its routes answering 404.
func wireOptionalStores(app *dataentry.App, svc *appbuild.Services) error {
	// Piles are absent when the backend offers no piles store; the routes
	// then 404 and the bootstrap says piles_available false.
	if p := svc.Piles(); p != nil {
		if err := app.SetPiles(p, appbuild.LuaPiles(svc)); err != nil {
			return fmt.Errorf("wire piles: %w", err)
		}
	}
	// Without a lease service the lease routes 404 and no response carries
	// `_lease`.
	if l := appbuild.Leases(svc); l != nil {
		if err := dataentry.SetLeases(app, l); err != nil {
			return fmt.Errorf("wire leases: %w", err)
		}
	}

	// Without record types in the schema the record routes answer 404.
	if rs := appbuild.Records(svc); rs != nil {
		if err := dataentry.SetRecords(app, rs, appbuild.LuaRecords(svc)); err != nil {
			return fmt.Errorf("wire records: %w", err)
		}
		if err := dataentry.SetRecordRollups(app, appbuild.RecordRollups(svc)); err != nil {
			return fmt.Errorf("wire record roll-ups: %w", err)
		}
	}

	// Without a database audit log (the fs and memory tiers) the audit
	// routes answer 404 and the sidebar offers neither audit view.
	if l := svc.AuditLog(); l != nil {
		if err := dataentry.SetAuditLog(app, l); err != nil {
			return fmt.Errorf("wire audit log: %w", err)
		}
	}

	// Without flows in flows.yaml the flow routes answer 404.
	if ctl := appbuild.FlowControlFor(svc); ctl != nil {
		if err := dataentry.SetFlows(app, ctl); err != nil {
			return fmt.Errorf("wire flows: %w", err)
		}
	}
	return nil
}
