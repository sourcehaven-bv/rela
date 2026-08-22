package migration

import (
	"slices"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&FormRelationDirectionMigration{})
}

// FormRelationDirectionMigration writes an explicit `direction:` onto form
// relation bindings that omit one, so the key stops being implicit.
//
// `direction` used to default to outgoing when absent. That default was wrong
// in two different ways. On a `to`-side binding it silently bound the wrong
// side of the edge (the widget searched the wrong entity type and never showed
// existing edges). On a self-referencing relation — the form's entity type is
// BOTH the from and the to, e.g. `depends-on` from ticket to ticket — it
// silently picked one of two equally valid, opposite readings.
//
// This migration fills in only the unambiguous half: bindings whose entity type
// sits on exactly ONE side, where there is a single correct answer. The
// self-referencing case is deliberately LEFT ALONE — the migration cannot know
// which reading the author meant, and writing a guess would cement it invisibly.
// Those are reported by ValidateConfig instead, which names each offending form
// and relation so a human can decide. That is why `rela migrate` is not always
// sufficient to make a project load: it is the easy-mode upgrade for the cases
// that have one right answer, not a rubber stamp over the ones that do not.
type FormRelationDirectionMigration struct {
	meta MetamodelProvider
}

// SetMetamodel implements MetamodelAware.
func (m *FormRelationDirectionMigration) SetMetamodel(meta MetamodelProvider) {
	m.meta = meta
}

func (m *FormRelationDirectionMigration) Name() string {
	return "form-relation-direction"
}

func (m *FormRelationDirectionMigration) Description() string {
	return "Write explicit direction: on form relations that omit it"
}

func (m *FormRelationDirectionMigration) FileTypes() []FileType {
	return []FileType{FileTypeDataEntry}
}

func (m *FormRelationDirectionMigration) Detect(doc *yaml.Node) bool {
	return m.walk(doc, false)
}

func (m *FormRelationDirectionMigration) Apply(doc *yaml.Node) error {
	m.walk(doc, true)
	return nil
}

// walk visits every form relation binding. With apply=false it reports whether
// any binding would change; with apply=true it writes the inferred directions.
func (m *FormRelationDirectionMigration) walk(doc *yaml.Node, apply bool) bool {
	if m.meta == nil {
		return false // without a metamodel nothing can be inferred
	}
	root := GetDocumentRoot(doc)
	if root == nil {
		return false
	}
	forms := GetMapValue(root, "forms")
	if forms == nil || forms.Kind != yaml.MappingNode {
		return false
	}

	changed := false
	for i := 1; i < len(forms.Content); i += 2 {
		formDef := forms.Content[i]
		if formDef.Kind != yaml.MappingNode {
			continue
		}
		entityType := getScalarValue(formDef, "entity_type")
		if entityType == "" {
			continue
		}
		// Flat relation list, plus each wizard step's own list.
		if m.walkRelations(GetMapValue(formDef, "relations"), entityType, apply) {
			changed = true
			if !apply {
				return true
			}
		}
		steps := GetMapValue(formDef, "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range steps.Content {
			if step.Kind != yaml.MappingNode {
				continue
			}
			if m.walkRelations(GetMapValue(step, "relations"), entityType, apply) {
				changed = true
				if !apply {
					return true
				}
			}
		}
	}
	return changed
}

func (m *FormRelationDirectionMigration) walkRelations(relations *yaml.Node, entityType string, apply bool) bool {
	if relations == nil || relations.Kind != yaml.SequenceNode {
		return false
	}
	changed := false
	for _, rel := range relations.Content {
		if rel.Kind != yaml.MappingNode {
			continue
		}
		dir, ok := m.inferDirection(rel, entityType)
		if !ok {
			continue
		}
		changed = true
		if !apply {
			return true
		}
		SetMapValue(rel, "direction", dir)
	}
	return changed
}

// inferDirection returns the direction to write for a binding, and whether it
// should be written at all.
//
// This mirrors dataentryconfig.InferDirection and must stay in lockstep with
// it. It cannot call that function directly: migrations walk yaml.Node against
// the narrow MetamodelProvider interface, not a *metamodel.Metamodel. It reports false when the binding already has a
// direction, names an unknown relation, or is ambiguous (entity type on both
// sides) — the last of which is left for the author to resolve.
func (m *FormRelationDirectionMigration) inferDirection(rel *yaml.Node, entityType string) (string, bool) {
	relation := getScalarValue(rel, "relation")
	if relation == "" || getScalarValue(rel, "direction") != "" {
		return "", false
	}
	fromTypes := m.meta.GetRelationFrom(relation)
	toTypes := m.meta.GetRelationTo(relation)
	inFrom := slices.Contains(fromTypes, entityType)
	inTo := slices.Contains(toTypes, entityType)
	switch {
	case inFrom && inTo:
		return "", false // ambiguous: only the author knows which was meant
	case inFrom:
		return "outgoing", true
	case inTo:
		return "incoming", true
	default:
		return "", false // on neither side; a wrong-side error, not a default
	}
}
