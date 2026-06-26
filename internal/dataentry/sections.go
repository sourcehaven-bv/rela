package dataentry

import (
	"context"
	"fmt"
	"strings"

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
type SectionFieldData struct {
	Property     string
	Label        string
	Values       []string
	PropType     string
	Inaccessible bool
}

// SectionEntityData holds a resolved entity for template rendering.
//
// `Props` and `FieldVerdicts` (TKT-IHC7D) carry the typed property
// values and per-cell writability verdicts for inline-edit hosts on
// cards/list view sections. Both are hidden-property-stripped. The
// wire converter dumb-copies them into V1ViewEntity._props and
// V1ViewEntity._fields respectively. They are nil for code paths that
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
	FieldVerdicts map[string]V1FieldAffordance
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
func (a *App) buildSectionEntityData(ctx context.Context, e *entity.Entity, secFields []ViewSectionField, eDef *metamodel.EntityDef) SectionEntityData {
	s := a.State()
	sed := SectionEntityData{
		ID:            e.ID,
		Title:         s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
		Type:          e.Type,
		EditFormID:    a.editFormForType(e.Type),
		Props:         a.affordances.copyVisibleProperties(ctx, e),
		FieldVerdicts: a.affordances.computeFieldAffordances(ctx, e),
	}
	for _, f := range secFields {
		values := propertyToStrings(e.Properties[f.Property])
		propType := ""
		if eDef != nil {
			if pd, ok := eDef.Properties[f.Property]; ok {
				propType = pd.Type
			}
		}
		label := f.Label
		if label == "" {
			label = titleCase(f.Property)
		}
		sed.Fields = append(sed.Fields, SectionFieldData{
			Property: f.Property, Label: label, Values: values, PropType: propType,
			Inaccessible: e.IsInaccessible(f.Property),
		})
	}
	return sed
}

// buildSections builds template-ready section data from view sections and a view result.
func (a *App) buildSections(ctx context.Context, sections []ViewSection, result *viewResult) []SectionData {
	s := a.State()
	out := make([]SectionData, 0, len(sections))

	for _, sec := range sections {
		sd := SectionData{
			Heading:      sec.Heading,
			SectionID:    slugify(sec.Heading),
			Display:      sec.Display,
			EmptyMessage: sec.EmptyMessage,
			Link:         sec.Link,
		}

		if sec.Source == "entry" {
			e := result.Entry
			entDef, _ := s.Meta.GetEntityDef(e.Type)

			switch sec.Display {
			case "properties":
				for _, f := range sec.Fields {
					values := propertyToStrings(e.Properties[f.Property])
					propType := ""
					if entDef != nil {
						if pd, ok := entDef.Properties[f.Property]; ok {
							propType = pd.Type
						}
					}
					label := f.Label
					if label == "" {
						label = titleCase(f.Property)
					}
					sd.Fields = append(sd.Fields, SectionFieldData{
						Property: f.Property, Label: label, Values: values, PropType: propType,
						Inaccessible: e.IsInaccessible(f.Property),
					})
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
					sed := a.buildSectionEntityData(ctx, e, sec.Fields, eDef)
					sd.Entities = append(sd.Entities, sed)
				}
			case "table":
				sd.Columns = sec.Columns
				buildRow := func(e *entity.Entity) SectionRowData {
					eDef, _ := s.Meta.GetEntityDef(e.Type)
					row := SectionRowData{EntityID: e.ID, EntityType: e.Type, EditFormID: a.editFormForType(e.Type)}
					for _, col := range sec.Columns {
						cell := SectionColumnData{
							Link: a.resolveLinkTarget(col.Link, e.Type, e.ID), EntityID: e.ID, EntityType: e.Type,
						}
						if col.Relation != "" {
							cell.Values = a.resolveRelationColumnValues(ctx, e.ID, col.Relation, col.Direction)
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
					sed := a.buildSectionEntityData(ctx, e, sec.Fields, eDef)
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
func (a *App) executeSidePanel(ctx context.Context, panel *SidePanelConfig, entityID, entityType string) []SectionData {
	if panel == nil || entityID == "" {
		return nil
	}

	// Build a synthetic ViewConfig to reuse executeView.
	viewCfg := ViewConfig{
		Entry:    ViewEntry{Type: entityType},
		Traverse: panel.Traverse,
		Sections: panel.Sections,
	}

	result, err := a.executeView(ctx, viewCfg, entityID)
	if err != nil {
		return nil
	}

	return a.buildSections(ctx, panel.Sections, result)
}

// resolveSectionButtonsWithTraverse populates AddInfo and LinkInfo on
// side-panel sections. The side panel is the only mutation surface that
// carries these affordances; the read-only entity-detail view path does
// not call this. The `viewConfig` parameter is a synthetic ViewConfig
// hand-built from a form's SidePanel config — it is not a generic view.
func (a *App) resolveSectionButtonsWithTraverse(viewConfig ViewConfig, sections []SectionData, entry *entity.Entity) {
	s := a.State()
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
				formID := a.createFormForType(et)
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
