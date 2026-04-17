package dataentry

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// SectionFieldData holds a single resolved field for template rendering.
type SectionFieldData struct {
	Label    string
	Value    string
	PropType string
}

// SectionEntityData holds a resolved entity for template rendering.
type SectionEntityData struct {
	ID         string
	Title      string
	Type       string
	EditFormID string
	Fields     []SectionFieldData
	Content    string
	HasContent bool
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

// buildSections builds template-ready section data from view sections and a view result.
func (a *App) buildSections(sections []ViewSection, result *viewResult) []SectionData {
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
					val := ""
					if v := e.Properties[f.Property]; v != nil {
						val = fmt.Sprintf("%v", v)
					}
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
						Label: label, Value: val, PropType: propType,
					})
				}
			case "content":
				sd.Content = e.Content
				sd.HasContent = e.Content != ""
			}
		} else {
			entities, exists := result.Collections[sec.Source]
			if !exists {
				entities = []*model.Entity{}
			}
			sd.IsEmpty = len(entities) == 0

			switch sec.Display {
			case "properties", "list":
				for _, e := range entities {
					eDef, _ := s.Meta.GetEntityDef(e.Type)
					sed := SectionEntityData{
						ID:         e.ID,
						Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
						Type:       e.Type,
						EditFormID: a.editFormForType(e.Type),
					}
					for _, f := range sec.Fields {
						val := ""
						if v := e.Properties[f.Property]; v != nil {
							val = fmt.Sprintf("%v", v)
						}
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
							Label: label, Value: val, PropType: propType,
						})
					}
					sd.Entities = append(sd.Entities, sed)
				}
			case "table":
				sd.Columns = sec.Columns
				buildRow := func(e *model.Entity) SectionRowData {
					eDef, _ := s.Meta.GetEntityDef(e.Type)
					row := SectionRowData{EntityID: e.ID, EntityType: e.Type, EditFormID: a.editFormForType(e.Type)}
					for _, col := range sec.Columns {
						cell := SectionColumnData{
							Link: a.resolveLinkTarget(col.Link, e.Type, e.ID), EntityID: e.ID, EntityType: e.Type,
						}
						if col.Relation != "" {
							cell.Values = a.resolveRelationColumnValues(e.ID, col.Relation, col.Direction)
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
					groups := map[string][]*model.Entity{}
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
						sortEntitiesByID(groups[gName])
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
					sed := SectionEntityData{
						ID:         e.ID,
						Title:      s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
						Type:       e.Type,
						EditFormID: a.editFormForType(e.Type),
						Content:    e.Content,
						HasContent: e.Content != "",
					}
					for _, f := range sec.Fields {
						val := ""
						if v := e.Properties[f.Property]; v != nil {
							val = fmt.Sprintf("%v", v)
						}
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
							Label: label, Value: val, PropType: propType,
						})
					}
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
func (a *App) executeSidePanel(panel *SidePanelConfig, entityID, entityType string) []SectionData {
	if panel == nil || entityID == "" {
		return nil
	}

	// Build a synthetic ViewConfig to reuse executeView.
	viewCfg := ViewConfig{
		Entry:    ViewEntry{Type: entityType},
		Traverse: panel.Traverse,
		Sections: panel.Sections,
	}

	result, err := a.executeView(viewCfg, entityID)
	if err != nil {
		return nil
	}

	return a.buildSections(panel.Sections, result)
}

// resolveSectionButtonsWithTraverse populates AddInfo and LinkInfo using full view config.
func (a *App) resolveSectionButtonsWithTraverse(viewConfig ViewConfig, sections []SectionData, entry *model.Entity) {
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
