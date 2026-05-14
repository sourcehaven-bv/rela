package dataentry

import (
	"sort"

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

// buildDefaultViewConfig synthesizes a ViewConfig for entityType when no
// explicit one is registered, so the unified detail screen always has
// something to render.
//
// Sections in order:
//  1. properties — every property in EntityDef.PropertyOrder
//  2. content   — the entity's markdown body
//  3. one section per outgoing relation whose From[] includes entityType
//  4. one section per incoming relation whose To[] includes entityType
//
// Relation sections iterate metamodel.Relations in alphabetical order so
// the output is deterministic across runs.
func buildDefaultViewConfig(meta *metamodel.Metamodel, entityType string) (ViewConfig, bool) {
	entDef, ok := meta.GetEntityDef(entityType)
	if !ok {
		return ViewConfig{}, false
	}

	view := ViewConfig{
		Title: entDef.Label,
		Entry: ViewEntry{Type: entityType},
	}

	// Properties section — only emitted when the entity has any.
	if len(entDef.PropertyOrder) > 0 {
		fields := make([]ViewSectionField, 0, len(entDef.PropertyOrder))
		for _, prop := range entDef.PropertyOrder {
			fields = append(fields, ViewSectionField{Property: prop})
		}
		view.Sections = append(view.Sections, ViewSection{
			Heading: "Properties",
			Source:  "entry",
			Display: "properties",
			Fields:  fields,
		})
	}

	// Content section — always emitted; the executor sets HasContent=false
	// when the body is empty, and the frontend hides empty content sections.
	view.Sections = append(view.Sections, ViewSection{
		Source:  "entry",
		Display: "content",
	})

	// Relation sections — sort relation names so the output is stable.
	relationNames := make([]string, 0, len(meta.Relations))
	for name := range meta.Relations {
		relationNames = append(relationNames, name)
	}
	sort.Strings(relationNames)

	// Outgoing first, then incoming.
	for _, name := range relationNames {
		def := meta.Relations[name]
		if !containsString(def.From, entityType) {
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
		// Symmetric relations are already covered by the outgoing pass —
		// emitting an incoming section would duplicate the same edges.
		if def.Symmetric {
			continue
		}
		// Self-referential relation: To[] also contains entityType. The
		// outgoing pass already emitted a section; emitting another one
		// for the inverse direction is the right behavior here, since the
		// edges visible to the inverse are different from the outgoing
		// ones. Allow through.
		if !containsString(def.To, entityType) {
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
