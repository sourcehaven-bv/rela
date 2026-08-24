package dataentry

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// propertyToStrings normalises a property value into a slice of non-empty
// strings. Handles scalars, []string, and []any (the three shapes markdown
// frontmatter can produce). nil or empty input returns an empty slice.
func propertyToStrings(v any) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s := fmt.Sprintf("%v", item)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		s := fmt.Sprintf("%v", t)
		if s == "" {
			return nil
		}
		return []string{s}
	}
}

// SectionFieldData holds a single resolved field for template rendering.
// Values is always a list so that list-typed properties (list: true in the
// metamodel) retain per-item structure; scalar properties become a 1-element
// slice. Empty properties emit an empty slice.
//
// Property is the raw property name (e.g. "title"); Label is its
// human-readable form. Inaccessible is true when the underlying entity is
// git-crypt encrypted and the value cannot be read with the current key —
// frontends render a lock indicator instead of the (absent) value.
// Span is the field's width on the 12-column layout grid, carried through from
// the view config. 0 means full width — the default for every auto-generated
// view, so a section with no spans authored renders as one scannable column.
//
// Render is the resolved render mode ("display" | "input", TKT-HOIX1) — see
// dataentryconfig.ResolveFieldRender. Resolved server-side so the SPA never
// reimplements the section→field inheritance rule.
type SectionFieldData struct {
	Property     string
	Label        string
	Values       []string
	PropType     string
	Inaccessible bool
	Span         int
	Render       string
	// Widget is the config's widget override for this field, empty when the
	// author did not set one. Passed through verbatim: resolving it is the
	// SPA's job (its registry owns the type→widget default), and the server
	// has already rejected a name/type mismatch at config load (TKT-3R7RF3).
	Widget string
}

// buildSectionFieldData resolves one configured field against an entity.
//
// Shared by both construction sites — buildSectionEntityData (cards/list rows)
// and the entry-source `properties` branch of buildSections (the detail page).
// They were near-identical literals, which is exactly how a new field like Span
// ends up wired into one and silently dropped from the other.
//
// sectionRender is the containing section's `render:` default; the field's own
// `render:` overrides it (TKT-HOIX1).
func buildSectionFieldData(
	f ViewSectionField, e *entity.Entity, eDef *metamodel.EntityDef,
	sectionRender string,
) SectionFieldData {
	propType := ""
	if eDef != nil {
		if pd, ok := eDef.Properties[f.Property]; ok {
			propType = pd.Type
		}
	}
	// A label is authored, never derived (DEC-6C1NAA): an unset label falls
	// back to the raw property name rather than a title-cased guess, which
	// would bake an English convention into a language-neutral metamodel.
	label := f.Label
	if label == "" {
		label = f.Property
	}
	return SectionFieldData{
		Property:     f.Property,
		Label:        label,
		Values:       propertyToStrings(e.Properties[f.Property]),
		PropType:     propType,
		Inaccessible: e.IsInaccessible(f.Property),
		// int, not dataentryconfig.Span: the named type exists to enforce
		// strict YAML decoding at config load. Past that boundary the value is
		// just a number, and the wire DTO shouldn't drag a config type onto
		// the API surface.
		Span:   int(f.Span),
		Render: resolveFieldRender(sectionRender, f.Render),
		// No section-level inheritance for Widget, unlike Render — a widget is
		// per-property, and a section-wide one would be a config error on every
		// field of a non-matching type. See ViewSectionField's godoc.
		Widget: f.Widget,
	}
}

// SectionEntityData holds a resolved entity for template rendering.
//
// `Props` and `FieldVerdicts` (TKT-IHC7D) carry the typed property
// values and per-cell writability verdicts for inline-edit hosts on
// cards/list view sections. Both are hidden-property-stripped. The
// wire converter dumb-copies them into v1.ViewEntity._props and
// v1.ViewEntity._fields respectively. They are nil for code paths that
// don't compute them (notably the entry-source branch and table rows);
// the wire converter's nil-checks gate emission.
type SectionEntityData struct {
	ID            string
	Title         string
	Type          string
	EditFormID    string
	Fields        []SectionFieldData
	Content       string
	HasContent    bool
	Props         map[string]any
	FieldVerdicts map[string]v1.FieldAffordance
	// World is the face-provenance of this entity under the view's world
	// (TKT-WRLDAPI item 4b). Nil under the default world.
	World *v1.EntityWorld
}

// SectionColumnData holds a resolved table cell for template rendering.
type SectionColumnData struct {
	Values     []string
	PropType   string
	Widget     string
	Link       string // resolved link URL or empty
	EntityID   string
	EntityType string
}

// SectionRowData holds a resolved table row for template rendering.
type SectionRowData struct {
	EntityID   string
	EntityType string
	EditFormID string
	Cells      []SectionColumnData
	Content    string
}

// GroupData holds a group of rows/entities for grouped table/card display.
type GroupData struct {
	GroupName string
	Rows      []SectionRowData
	Entities  []SectionEntityData
}

// SectionAddTarget holds a possible entity type target for an "Add" button.
type SectionAddTarget struct {
	EntityType string
	FormID     string
	Label      string
}

// SectionAddInfo describes an "Add" button on a view section.
type SectionAddInfo struct {
	Relation string
	LinkAs   string // "from" or "to" — role of the new entity in the relation
	PeerID   string // entry entity ID
	Targets  []SectionAddTarget
}

// SectionLinkInfo describes a "Link existing" button on a view section.
type SectionLinkInfo struct {
	Relation    string   // relation type name
	LinkAs      string   // "from" or "to" — role of the linked entity
	PeerID      string   // entry entity ID
	EntityTypes []string // valid target entity types
}

// SectionData holds all resolved data for a single view section.
type SectionData struct {
	Heading      string
	SectionID    string
	Display      string
	Fields       []SectionFieldData
	Entities     []SectionEntityData
	Columns      []ListColumn
	Rows         []SectionRowData
	Groups       []GroupData
	IsGrouped    bool
	EmptyMessage string
	IsEmpty      bool
	Link         string // section-level link configuration (currently unused in templates)
	Content      string
	HasContent   bool
	AddInfo      *SectionAddInfo
	LinkInfo     *SectionLinkInfo
}

// buildSectionEntityData composes the per-row data for a cards/list
// view section (TKT-IHC7D). Both non-entry display modes — `properties`
// / `list` and `content` / `cards` — call this so the typed `_props`
// and `_fields` wire surfaces stay consistent across modes.
//
// Returns a value (not a pointer) so callers can layer on display-mode-
// specific fields (e.g. `Content`/`HasContent` for the `content`/`cards`
// branch) without sharing mutation across rows.
//
// sectionRender is the containing section's `render:` default; each field's
// own `render:` overrides it (TKT-HOIX1).
//
// w is the world the containing view executed in; it labels each row's face
// provenance (TKT-WRLDAPI item 4b) and is nil-stamped under the default world.
func (h *viewsHandler) buildSectionEntityData(
	ctx context.Context, e *entity.Entity, secFields []ViewSectionField, eDef *metamodel.EntityDef,
	sectionRender string, w viewWorld,
) SectionEntityData {
	s := h.schema()
	sed := SectionEntityData{
		ID:            e.ID,
		Title:         s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
		Type:          e.Type,
		EditFormID:    h.editFormForType(e.Type),
		Props:         h.affordances.copyVisibleProperties(ctx, e),
		FieldVerdicts: h.affordances.computeFieldAffordances(ctx, e),
		World:         w.provenanceFor(e),
	}
	for _, f := range secFields {
		sed.Fields = append(sed.Fields, buildSectionFieldData(f, e, eDef, sectionRender))
	}
	return sed
}

// buildSections builds template-ready section data from view sections and a view result.
//
//nolint:gocognit,funlen // builds each section by its declared source and display mode; the branches are the distinct section kinds, not shared logic to extract.
func (h *viewsHandler) buildSections(ctx context.Context, sections []ViewSection, result *viewResult) []SectionData {
	s := h.schema()
	out := make([]SectionData, 0, len(sections))

	for _, sec := range sections {
		sd := SectionData{
			Heading:      sec.Heading,
			SectionID:    slugify(sec.Heading),
			Display:      sec.Display,
			EmptyMessage: sec.EmptyMessage,
			Link:         sec.Link,
		}

		if sec.Source == "entry" { //nolint:nestif // entry-source sections branch by display mode and property shape; the nesting is the per-mode build, not extractable logic.
			e := result.Entry
			entDef, _ := s.Meta.GetEntityDef(e.Type)

			switch sec.Display {
			case "properties":
				for _, f := range sec.Fields {
					sd.Fields = append(sd.Fields, buildSectionFieldData(f, e, entDef, sec.Render))
				}
			case "content":
				sd.Content = e.Content
				sd.HasContent = e.Content != ""
			}
		} else {
			entities, exists := result.Collections[sec.Source]
			if !exists {
				entities = []*entity.Entity{}
			}
			sd.IsEmpty = len(entities) == 0

			switch sec.Display {
			case "properties", "list":
				for _, e := range entities {
					eDef, _ := s.Meta.GetEntityDef(e.Type)
					sed := h.buildSectionEntityData(ctx, e, sec.Fields, eDef, sec.Render, result.World)
					sd.Entities = append(sd.Entities, sed)
				}
			case "table":
				sd.Columns = sec.Columns
				buildRow := func(e *entity.Entity) SectionRowData {
					eDef, _ := s.Meta.GetEntityDef(e.Type)
					row := SectionRowData{EntityID: e.ID, EntityType: e.Type, EditFormID: h.editFormForType(e.Type)}
					for _, col := range sec.Columns {
						cell := SectionColumnData{
							Link: resolveLinkTarget(col.Link, e.Type, e.ID), EntityID: e.ID, EntityType: e.Type,
						}
						if col.Relation != "" {
							cell.Values = h.resolveRelationColumnValues(ctx, e.ID, col.Relation, col.Direction)
						} else {
							var pd metamodel.PropertyDef
							if eDef != nil {
								if propDef, ok := eDef.Properties[col.Property]; ok {
									pd = propDef
									cell.PropType = pd.Type
								}
							}
							cell.Widget = resolveWidget(pd, s.Meta)
							if vs := e.GetAttributeStrings(col.Property); vs != nil {
								cell.Values = vs
							} else if val := e.GetAttributeString(col.Property); val != "" {
								cell.Values = []string{val}
							}
						}
						row.Cells = append(row.Cells, cell)
					}
					return row
				}
				if sec.GroupBy != "" {
					sd.IsGrouped = true
					groups := map[string][]*entity.Entity{}
					var groupOrder []string
					for _, e := range entities {
						prop := strings.TrimPrefix(sec.GroupBy, "properties.")
						groupKey := "(none)"
						if v := e.Properties[prop]; v != nil {
							groupKey = fmt.Sprintf("%v", v)
						}
						if _, seen := groups[groupKey]; !seen {
							groupOrder = append(groupOrder, groupKey)
						}
						groups[groupKey] = append(groups[groupKey], e)
					}
					for _, gName := range groupOrder {
						gd := GroupData{GroupName: gName}
						sortStoreEntitiesByID(groups[gName])
						for _, e := range groups[gName] {
							gd.Rows = append(gd.Rows, buildRow(e))
						}
						sd.Groups = append(sd.Groups, gd)
					}
				} else {
					for _, e := range entities {
						sd.Rows = append(sd.Rows, buildRow(e))
					}
				}

			case "content", "cards":
				for _, e := range entities {
					eDef, _ := s.Meta.GetEntityDef(e.Type)
					sed := h.buildSectionEntityData(ctx, e, sec.Fields, eDef, sec.Render, result.World)
					sed.Content = e.Content
					sed.HasContent = e.Content != ""
					sd.Entities = append(sd.Entities, sed)
				}
			}
		}

		out = append(out, sd)
	}

	return out
}

// executeSidePanel runs the side panel traversal and builds section data.
// Returns nil if the form has no side panel or the entity doesn't exist.
func (h *viewsHandler) executeSidePanel(
	ctx context.Context, panel *SidePanelConfig, entityID, entityType string,
) []SectionData {
	if panel == nil || entityID == "" {
		return nil
	}

	// Build a synthetic ViewConfig to reuse executeView.
	viewCfg := ViewConfig{
		Entry:    ViewEntry{Type: entityType},
		Traverse: panel.Traverse,
		Sections: panel.Sections,
	}

	// DEFAULT WORLD, named explicitly. The side panel has not been scoped for
	// worlds (TKT-WRLDAPI item 4b covers `_views` only), and `_sidepanel` is
	// refused a `?world=` by worldCapablePath — but a surface must DECLARE its
	// world rather than inherit one, or scoping `_views` silently drags every
	// executeView caller along with it.
	result, err := h.executeView(ctx, viewCfg, entityID, defaultViewWorld())
	if err != nil {
		return nil
	}

	return h.buildSections(ctx, panel.Sections, result)
}

// resolveSectionButtonsWithTraverse populates AddInfo and LinkInfo on
// side-panel sections. The side panel is the only mutation surface that
// carries these affordances; the read-only entity-detail view path does
// not call this. The `viewConfig` parameter is a synthetic ViewConfig
// hand-built from a form's SidePanel config — it is not a generic view.
//
//nolint:gocognit // resolves section buttons across traverse targets; the branches are per-source button-resolution cases, not shared logic to extract.
func (h *viewsHandler) resolveSectionButtonsWithTraverse(
	viewConfig ViewConfig, sections []SectionData, entry *entity.Entity,
) {
	s := h.schema()
	for i, sec := range viewConfig.Sections {
		if sec.Source == "entry" {
			continue
		}
		for _, rule := range viewConfig.Traverse {
			if rule.CollectAs != sec.Source || rule.From != "entry" {
				continue
			}
			relName := rule.Follow
			linkAs := "to" // new entity is the target (outgoing from entry)
			if rule.FollowIncoming != "" {
				relName = rule.FollowIncoming
				linkAs = "from" // new entity is the source (incoming to entry)
			}
			relDef, ok := s.Meta.GetRelationDef(relName)
			if !ok {
				break
			}
			// Determine valid target types for creation
			var candidateTypes []string
			if linkAs == "to" {
				candidateTypes = relDef.To
			} else {
				candidateTypes = relDef.From
			}
			var targets []SectionAddTarget
			for _, et := range candidateTypes {
				formID := h.createFormForType(et)
				if formID == "" {
					continue
				}
				label := et
				if ed, ok := s.Meta.GetEntityDef(et); ok && ed.Label != "" {
					label = ed.Label
				}
				targets = append(targets, SectionAddTarget{
					EntityType: et, FormID: formID, Label: label,
				})
			}
			if len(targets) > 0 {
				sections[i].AddInfo = &SectionAddInfo{
					Relation: relName,
					LinkAs:   linkAs,
					PeerID:   entry.ID,
					Targets:  targets,
				}
			}
			// Link existing: always available when candidate types exist
			if len(candidateTypes) > 0 {
				sections[i].LinkInfo = &SectionLinkInfo{
					Relation:    relName,
					LinkAs:      linkAs,
					PeerID:      entry.ID,
					EntityTypes: candidateTypes,
				}
			}
			break
		}
	}
}
