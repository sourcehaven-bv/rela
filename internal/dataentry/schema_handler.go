package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// schemaHandler owns the configuration endpoints: /_schema, /_schema/...,
// /_config, the OpenAPI spec and /_templates/{type}. Everything it serves is
// operator-authored configuration, which is not a secret (root CLAUDE.md), so
// none of it is filtered per principal. Extracted from App to take that
// cluster off the god-object.
//
// The collaborators are closures over App rather than captured values: tests
// and the wiring setters reassign App.store, App.gitOps, App.channels,
// App.versions, App.collab, App.palette and App.assets after construction,
// and a captured copy would go stale (the gitHandler idiom).
type schemaHandler struct {
	// schema is the reloadable snapshot. Each request takes it once.
	schema func() *Schema
	// store is the store the app was wired with; it fixes the advertised
	// capabilities (TKT-ZMPDZ3R).
	store func() store.Store
	// gitEnabled reports whether git operations are configured.
	gitEnabled func() bool
	// flowsEnabled reports whether the flows routes are served.
	flowsEnabled func() bool
	// channels describes the channel routes for /_schema/channels.
	channels func() channelDescriber
	// historyEnabled reports whether content versioning is wired.
	historyEnabled func() bool
	// collab is the co-editing relay; nil when co-editing is not served.
	collab func() *collabHandler
	// palette resolves the served palette.
	palette func() *paletteService
	// assets reads the project's apps/ directory.
	assets func() projectAssets
	// templater loads the entity templates for /_templates. Nil in tests
	// that never serve that route.
	templater templating.Templater
}

// newSchemaHandler wires the handler to app's live collaborators.
func newSchemaHandler(app *App, templater templating.Templater) *schemaHandler {
	return &schemaHandler{
		schema:         app.State,
		store:          func() store.Store { return app.store },
		gitEnabled:     func() bool { return app.gitOps != nil },
		flowsEnabled:   func() bool { return app.flows != nil },
		channels:       func() channelDescriber { return app.channels.describer() },
		historyEnabled: func() bool { return app.versions != nil },
		collab:         func() *collabHandler { return app.collab },
		palette:        func() *paletteService { return app.palette },
		assets:         func() projectAssets { return app.assets },
		templater:      templater,
	}
}

// scanAppsOrLog scans the project's apps/ directory, logging (not failing) on
// error so a transient scan problem degrades to "no apps" rather than breaking
// the whole config response.
func (h *schemaHandler) scanAppsOrLog(ctx context.Context) []appInfo {
	apps, err := scanApps(ctx, h.assets())
	if err != nil {
		slog.Warn("scanning apps directory failed", "error", err)
		return nil
	}
	return apps
}

//archplan:auth=required reason="metamodel; config is not a secret"
func (h *schemaHandler) handleV1Schema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	s := h.schema()
	st := h.store()
	schema := v1.Schema{
		Entities:   make(map[string]v1.EntityType),
		Relations:  make(map[string]v1.RelationType),
		Types:      make(map[string]v1.CustomType),
		Worlds:     schemaWorlds(r.Context(), s.Meta),
		WorldOrder: metamodel.WorldOrderOf(s.Meta),
		// Fixed when the app is wired (TKT-ZMPDZ3R).
		Capabilities: v1.Capabilities{
			AtomicBatch: store.TxRollsBack(st),
			AsOf:        store.SupportsReadInstant(st),
			Git:         h.gitEnabled(),
			Flows:       h.flowsEnabled(),
		},
	}

	for name, def := range s.Meta.Entities {
		schema.Entities[name] = toV1EntityType(s.Meta, name, def)
	}

	for name, def := range s.Meta.Relations {
		rt := v1.RelationType{
			Label:       def.Label,
			Description: def.Description,
			From:        def.From,
			To:          def.To,
			Symmetric:   def.Symmetric,
			Owning:      def.Owning,
			MinOutgoing: def.MinOutgoing,
			MaxOutgoing: def.MaxOutgoing,
			MinIncoming: def.MinIncoming,
			MaxIncoming: def.MaxIncoming,
		}
		if def.Inverse != nil && def.Inverse.ID != "" {
			rt.Inverse = &v1.InverseDef{ID: def.Inverse.ID, Label: def.Inverse.Label}
		}
		if def.Scope.IsContent() {
			rt.Scope = "content"
		}
		if len(def.Properties) > 0 {
			rt.Properties = make(map[string]v1.PropertyDef, len(def.Properties))
			for propName, propDef := range def.Properties {
				rt.Properties[propName] = toV1PropertyDef(s.Meta, propDef)
			}
		}
		if def.OutgoingOrderProperty() != "" || def.IncomingOrderProperty() != "" {
			rt.Orderable = &v1.RelationOrderable{
				Outgoing: def.OutgoingOrderProperty() != "",
				Incoming: def.IncomingOrderProperty() != "",
			}
		}
		schema.Relations[name] = rt
	}

	for name, def := range s.Meta.Types {
		schema.Types[name] = toV1CustomType(def)
	}

	schema.Abstract = toV1AbstractTypes(s.Meta)

	writeV1JSON(w, http.StatusOK, schema)
}

//archplan:auth=required reason="metamodel; config is not a secret"
func (h *schemaHandler) handleV1SchemaRoutes(w http.ResponseWriter, r *http.Request) {
	s := h.schema()
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_schema/")

	switch {
	case path == "types":
		// List entity type names
		names := make([]string, 0, len(s.Meta.Entities))
		for name := range s.Meta.Entities {
			names = append(names, name)
		}
		sort.Strings(names)
		writeV1JSON(w, http.StatusOK, names)

	case strings.HasPrefix(path, "types/"):
		// Get specific entity type
		typeName := strings.TrimPrefix(path, "types/")
		def, ok := s.Meta.Entities[typeName]
		if !ok {
			detail := ""
			if err := metamodel.CheckConcreteType(s.Meta, typeName); err != nil {
				detail = err.Error()
			}
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity type not found", detail)
			return
		}
		writeV1JSON(w, http.StatusOK, toV1EntityType(s.Meta, typeName, def))

	case path == "relations":
		writeV1JSON(w, http.StatusOK, s.Meta.Relations)

	case path == "channels":
		serveChannelSchema(w, r, h.channels())

	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
	}
}

// resolveFormRelations returns a copy of rels with server-derived defaults
// filled in, for both the flat and wizard-step relation lists in handleV1Config.
//
// Widget: an empty Widget becomes "cards" when the relation type has edge
// properties/content.
//
// Direction: an absent `direction:` is inferred from the metamodel — the SPA
// widgets test `direction === 'incoming'` literally (RelationCards.vue,
// RelationPicker.vue), so the inference MUST happen here rather than in the
// browser, or a to-side binding would validate clean on the server and still
// render the wrong side. Resolving once on the way out keeps the two consumers
// agreeing by construction. The ambiguous case (entity type on both sides) is
// rejected by ValidateConfig at load, so it cannot reach this point.
func resolveFormRelations(
	s *Schema, entityType string, rels []dataentryconfig.FormRelation,
) []dataentryconfig.FormRelation {
	resolved := make([]dataentryconfig.FormRelation, len(rels))
	copy(resolved, rels)
	for i := range resolved {
		if resolved[i].Widget == "" {
			if def, ok := s.Meta.GetRelationDef(resolved[i].Relation); ok && def.HasAdvancedFeatures() {
				resolved[i].Widget = WidgetCards
			}
		}
		resolved[i].Direction = resolveConfigDirection(
			s, entityType, resolved[i].Relation, resolved[i].Direction)
	}
	return resolved
}

// resolveConfigDirection fills in an absent `direction:` from the metamodel,
// for any relation binding anchored to entityType.
//
// Unrelated to [resolveDirection], which maps a PATCH wire key to a canonical
// relation name; this one is about config bindings served to the SPA.
//
// Every SPA consumer of a direction tests the literal string (`direction ===
// 'incoming'` in RelationCards, RelationPicker, FilterBar, EntityList and
// KanbanView), so an inferred direction has to be materialized here or the
// browser silently falls back to outgoing. Validation rejects the ambiguous
// case at load, so only Resolved changes anything; everything else keeps the
// historical reading, which is also what the SPA would have assumed.
func resolveConfigDirection(
	s *Schema, entityType, relation string, dir dataentryconfig.Direction,
) dataentryconfig.Direction {
	if dir != "" || relation == "" {
		return dir
	}
	inferred, res := dataentryconfig.InferDirection(entityType, relation, s.Meta)
	switch res {
	case dataentryconfig.DirectionResolved:
		return inferred
	case dataentryconfig.DirectionAmbiguous:
		// ValidateConfig rejects an ambiguous binding at load, so the app
		// should not have started. Reaching here means that gate was bypassed;
		// say so rather than silently picking a side.
		slog.Warn("relation binding direction is ambiguous but reached the config handler; serving outgoing",
			"entity_type", entityType, "relation", relation)
	case dataentryconfig.DirectionNoSide, dataentryconfig.DirectionUnknown:
		// A wrong-side or unreadable binding — both already reported by
		// validation. Serve the historical reading rather than inventing one;
		// the SPA needs a non-empty value either way.
	}
	return dataentryconfig.DirectionOutgoing
}

// resolveListDirections returns a copy of lists with relation-column and
// filter-control directions materialized, and empty column labels filled
// from the schema (withSchemaLabels). Copy-on-serve, like
// resolveFormRelations: the in-memory config keeps what the operator wrote.
func resolveListDirections(s *Schema, lists map[string]dataentryconfig.List) map[string]dataentryconfig.List {
	out := make(map[string]dataentryconfig.List, len(lists))
	for id, list := range lists {
		l := list
		if len(l.Columns) > 0 {
			cols := make([]dataentryconfig.ListColumn, len(l.Columns))
			copy(cols, l.Columns)
			for i := range cols {
				cols[i].Direction = resolveConfigDirection(s, l.EntityType, cols[i].Relation, cols[i].Direction)
			}
			l.Columns = withSchemaLabels(s.Meta, []string{l.EntityType}, cols)
		}
		l.FilterControls = resolveFilterControlDirections(s, l.EntityType, l.FilterControls)
		out[id] = l
	}
	return out
}

// resolveFilterControlDirections materializes directions on a filter-control
// list. Shared by lists and kanbans, which carry the same control type.
func resolveFilterControlDirections(
	s *Schema, entityType string, controls []dataentryconfig.FilterControl,
) []dataentryconfig.FilterControl {
	if len(controls) == 0 {
		return controls
	}
	out := make([]dataentryconfig.FilterControl, len(controls))
	copy(out, controls)
	for i := range out {
		out[i].Direction = resolveConfigDirection(s, entityType, out[i].Relation, out[i].Direction)
	}
	return out
}

// resolveKanbanDirections returns a copy of kanbans with card-field and
// filter-control directions materialized, and each cell color's style class.
func resolveKanbanDirections(s *Schema, kanbans map[string]dataentryconfig.Kanban) map[string]dataentryconfig.Kanban {
	var operator map[string]dataentryconfig.OperatorColor
	if s.Cfg != nil {
		operator = s.Cfg.OperatorColors()
	}
	out := make(map[string]dataentryconfig.Kanban, len(kanbans))
	for id, kanban := range kanbans {
		k := kanban
		k.CellColors = dataentryconfig.ResolveKanbanCellClasses(k.CellColors, operator)
		if len(k.Card.Fields) > 0 {
			fields := make([]dataentryconfig.KanbanCardField, len(k.Card.Fields))
			copy(fields, k.Card.Fields)
			for i := range fields {
				fields[i].Direction = resolveConfigDirection(s, k.EntityType, fields[i].Relation, fields[i].Direction)
			}
			k.Card.Fields = fields
		}
		k.FilterControls = resolveFilterControlDirections(s, k.EntityType, k.FilterControls)
		out[id] = k
	}
	return out
}

// resolveCalendarDirections fills in each calendar's relation directions, the
// same treatment resolveKanbanDirections gives a board (TKT-QXY8CQ).
//
// Calendars need it for the same reason: an event chip's relation field is
// resolved client-side against the relation's inverse key, and an unresolved
// direction leaves the SPA computing the wrong key and rendering nothing. The
// per-source entity type is what a field resolves against, so a chip field is
// resolved once per source rather than once per calendar — a calendar may
// project several types, and a relation's direction is only meaningful
// relative to one of them.
func resolveCalendarDirections(
	s *Schema, calendars map[string]dataentryconfig.Calendar,
) map[string]dataentryconfig.Calendar {
	out := make(map[string]dataentryconfig.Calendar, len(calendars))
	for id, calendar := range calendars {
		c := calendar
		// A field's direction depends on the type it hangs off, so resolve
		// against the first source that actually declares the relation.
		if len(c.Event.Fields) > 0 {
			fields := make([]dataentryconfig.KanbanCardField, len(c.Event.Fields))
			copy(fields, c.Event.Fields)
			for i := range fields {
				fields[i].Direction = resolveCalendarFieldDirection(s, c.Sources, fields[i])
			}
			c.Event = dataentryconfig.CalendarEvent{Fields: fields}
		}
		if len(c.Sources) > 0 && len(c.FilterControls) > 0 {
			c.FilterControls = resolveFilterControlDirections(s, c.Sources[0].EntityType, c.FilterControls)
		}
		out[id] = c
	}
	return out
}

// resolveCalendarFieldDirection resolves one chip field's direction against the
// calendar's sources, taking the first that yields a direction.
//
// Nil: an empty result is returned unchanged when no source declares the
// relation — the field simply renders nothing for those sources, which is the
// documented best-effort behavior for a heterogeneous calendar.
func resolveCalendarFieldDirection(
	s *Schema, sources []dataentryconfig.CalendarSource, field dataentryconfig.KanbanCardField,
) dataentryconfig.Direction {
	if field.Relation == "" {
		return field.Direction
	}
	for _, src := range sources {
		if d := resolveConfigDirection(s, src.EntityType, field.Relation, field.Direction); d != "" {
			return d
		}
	}
	return field.Direction
}

// aboutDescription is the deployment description shown by the SPA's global
// "About" help (TKT-DUQBD0). The data-entry.yaml `app.description` wins when set
// (it is the UI-app-specific text); otherwise it falls back to the metamodel's
// top-level `description` (the schema-level prose added in TKT-0YBFT8). Empty
// when neither is set — the SPA hides the About button.
//
// This is a SEPARATE field from AppConfig.Description on the wire: that field is
// also rendered by SettingsView as a plain one-line value, and pushing the
// (possibly multi-paragraph markdown) metamodel description into it would
// regress that view. The About surface gets its own field, `about_description`.
func aboutDescription(s *Schema) string {
	if s.Cfg != nil && s.Cfg.App.Description != "" {
		return s.Cfg.App.Description
	}
	if s.Meta != nil {
		return s.Meta.Description
	}
	return ""
}

// aboutWire projects `app.about:`. Each link is re-checked with the load-time
// rule, because a live config reload does not re-run validation; an invalid
// link is dropped rather than served, as meLinks does.
func aboutWire(a dataentryconfig.AboutConfig) *v1.AppAbout {
	out := v1.AppAbout{Version: a.Version}
	for _, l := range a.Links {
		if strings.TrimSpace(l.Label) != "" && dataentryconfig.ValidAccountLink(l.URL) {
			out.Links = append(out.Links, v1.AppAboutLink{Label: l.Label, URL: l.URL})
		}
	}
	if out.Version == "" && len(out.Links) == 0 {
		return nil
	}
	return &out
}

//archplan:auth=required reason="navigation config, served verbatim"
func (h *schemaHandler) handleV1Config(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	s := h.schema()
	collab := h.collab()
	// Resolve relation widgets: auto-detect "cards" for relations with
	// properties/content, for both single-page (flat) and wizard (step) forms.
	forms := make(map[string]dataentryconfig.Form, len(s.Cfg.Forms))
	for id, form := range s.Cfg.Forms {
		f := form
		f.Relations = resolveFormRelations(s, form.EntityType, f.Relations)
		if len(f.Steps) > 0 {
			steps := make([]dataentryconfig.FormStep, len(f.Steps))
			copy(steps, f.Steps)
			for i := range steps {
				steps[i].Relations = resolveFormRelations(s, form.EntityType, steps[i].Relations)
			}
			f.Steps = steps
		}
		forms[id] = f
	}

	config := v1.Config{
		NextActionBands: s.Cfg.NextActionBands,
		Piles:           pilesWire(s.Cfg.Piles),
		App: v1.AppConfig{
			Name:              s.Cfg.App.Name,
			Description:       s.Cfg.App.Description,
			Language:          s.Cfg.App.Language,
			PlantUMLServerURL: s.Cfg.App.PlantUMLServerURL,
			DefaultWorld:      declaredDefaultWorld(s.Meta),
			WorldSwitcher:     worldSwitcherWire(s.Cfg.App.WorldSwitcher),
			// The same nil check the history handler gates on, so the
			// affordance and the endpoint cannot disagree.
			HistoryEnabled: h.historyEnabled(),
			CollabEnabled:  collab != nil,
			CollabEpoch:    collabEpoch(collab),
			About:          aboutWire(s.Cfg.App.About),
		},
		AboutDescription: aboutDescription(s),
		Styles:           s.StyleMap,
		Forms:            forms,
		Channels:         s.Cfg.Channels,
		Lists:            resolveListDirections(s, s.Cfg.Lists),
		Views:            s.Cfg.Views,
		EntityViews:      s.Cfg.EntityViews,
		Kanbans:          resolveKanbanDirections(s, s.Cfg.Kanbans),
		Calendars:        resolveCalendarDirections(s, s.Cfg.Calendars),
		Gantts:           s.Cfg.Gantts,
		Sankeys:          s.Cfg.Sankeys,
		Dashboard:        s.Cfg.Dashboard,
		Dashboards:       s.Cfg.Dashboards,
		Actions:          s.Cfg.Actions,
		Navigation:       s.Cfg.Navigation,
		Spaces:           s.Cfg.Spaces,
		Builtins:         s.Cfg.Builtins,
		Pages:            s.Cfg.Pages,
		Documents:        s.Cfg.Documents,
		Apps:             appsToV1(h.scanAppsOrLog(r.Context())),
		Palette:          h.palette().Resolved(),
	}

	writeV1JSON(w, http.StatusOK, config)
}

// handleV1Templates returns templates for an entity type.
// GET /api/v1/_templates/{entityType}
//
//archplan:auth=required
func (h *schemaHandler) handleV1Templates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	} // Extract entity type from path: /api/v1/_templates/{entityType}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_templates/")
	entityType := strings.TrimSuffix(path, "/")

	if entityType == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_entity_type", "Entity type is required", "")
		return
	}

	// Check if entity type exists
	meta := h.schema().Meta
	if _, ok := meta.Entities[entityType]; !ok {
		writeV1Error(w, r, http.StatusNotFound, "entity_type_not_found",
			fmt.Sprintf("Entity type '%s' not found", entityType), "")
		return
	}

	templates, _ := templating.EntityTemplatesFor(r.Context(), h.templater, meta, entityType)
	result := make([]v1.Template, 0, len(templates))

	for _, t := range templates {
		relations := make([]v1.TemplateRelation, 0, len(t.Relations))
		for _, rel := range t.Relations {
			relations = append(relations, v1.TemplateRelation{
				Relation: rel.Type,
				Target:   rel.Target,
			})
		}
		result = append(result, v1.Template{
			Name:       t.Name,
			Properties: t.Properties,
			Content:    t.Content,
			Relations:  relations,
		})
	}

	writeV1JSON(w, http.StatusOK, result)
}

// --- OpenAPI Spec ---

// handleV1OpenAPI serves the OpenAPI 3.1 specification.
//
//archplan:auth=required reason="API description"
func (h *schemaHandler) handleV1OpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	data, err := h.schema().OpenAPIGen.GenerateJSON()
	if err != nil {
		writeInternalError(w, r, "generation_failed", "Failed to generate OpenAPI spec", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// no-cache, not no-store: a client may keep the spec (it is configuration,
	// not data) but must fetch it again before reuse, since it changes when the
	// schema is reloaded. There is no validator, so that fetch is a full one.
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}
