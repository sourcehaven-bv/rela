package dataentry

import (
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// findViewByEntityType returns the registered ViewConfig whose entry.type
// matches entityType. Validation enforces that at most one view targets a
// given entity type, so the first match is the only match.
func findViewByEntityType(views map[string]ViewConfig, entityType string) (ViewConfig, bool) {
	for _, v := range views {
		if v.Entry.Type == entityType {
			return v, true
		}
	}
	return ViewConfig{}, false
}

// buildDefaultViewConfig synthesizes a ViewConfig for entityType when none is
// registered. Sections in order: the properties in PropertyOrder (editable,
// no heading, title left out; the ACL still decides writability), the
// markdown content, one section per outgoing relation from entityType, and one
// per incoming relation to it. Relations iterate alphabetically so the output
// is deterministic.
func buildDefaultViewConfig(meta *metamodel.Metamodel, entityType string) (ViewConfig, bool) {
	entDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return ViewConfig{}, false
	}

	view := ViewConfig{
		Title: entDef.Label,
		Entry: ViewEntry{Type: entityType},
	}

	primary := entDef.GetPrimaryProperty()
	fields := make([]ViewSectionField, 0, len(entDef.PropertyOrder))
	for _, prop := range entDef.PropertyOrder {
		if prop == primary {
			continue
		}
		fields = append(fields, ViewSectionField{Property: prop})
	}
	if len(fields) > 0 {
		view.Sections = append(view.Sections, ViewSection{
			Source:  "entry",
			Display: "properties",
			Render:  dataentryconfig.RenderInput,
			Fields:  fields,
		})
	}

	view.Sections = append(view.Sections, ViewSection{
		Source:  "entry",
		Display: "content",
	})

	relationNames := make([]string, 0, len(meta.Relations))
	for name := range meta.Relations {
		relationNames = append(relationNames, name)
	}
	slices.Sort(relationNames)

	for _, name := range relationNames {
		def := meta.Relations[name]
		if !slices.Contains(def.From, entityType) {
			continue
		}
		collectAs := "out_" + name
		view.Traverse = append(view.Traverse, ViewTraverse{
			From:      "entry",
			Follow:    name,
			CollectAs: collectAs,
		})
		view.Sections = append(view.Sections, ViewSection{
			Heading: def.Label,
			Source:  collectAs,
			Display: "cards",
		})
	}
	for _, name := range relationNames {
		def := meta.Relations[name]
		if def.Symmetric {
			continue
		}
		if !slices.Contains(def.To, entityType) {
			continue
		}
		collectAs := "in_" + name
		view.Traverse = append(view.Traverse, ViewTraverse{
			From:           "entry",
			FollowIncoming: name,
			CollectAs:      collectAs,
		})
		heading := def.Label
		if def.Inverse != nil && def.Inverse.GetLabel() != "" {
			heading = def.Inverse.GetLabel()
		}
		view.Sections = append(view.Sections, ViewSection{
			Heading: heading,
			Source:  collectAs,
			Display: "cards",
		})
	}

	return view, true
}
