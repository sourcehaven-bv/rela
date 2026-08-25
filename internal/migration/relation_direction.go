package migration

import (
	"slices"

	"gopkg.in/yaml.v3"
)

func init() {
	Register(&RelationDirectionMigration{})
}

// RelationDirectionMigration writes an explicit `direction:` onto relation
// bindings that omit one, so the key stops being implicit.
//
// It covers every surface that carries a direction: form relations (including
// wizard steps), list columns and filter controls, kanban card fields and
// filter controls, and CalDAV dynamic collections.
//
// `direction` used to default to outgoing when absent. That default was wrong
// in two different ways. On a `to`-side binding it silently bound the wrong
// side of the edge (a form widget searched the wrong entity type and never
// showed existing edges; a CalDAV collection selected the wrong members). On a
// self-referencing relation — the binding's entity type is BOTH the from and
// the to, e.g. `depends-on` from ticket to ticket — it silently picked one of
// two equally valid, opposite readings.
//
// This migration fills in only the unambiguous half: bindings whose entity type
// sits on exactly ONE side, where there is a single correct answer. The
// self-referencing case is deliberately LEFT ALONE — the migration cannot know
// which reading the author meant, and writing a guess would cement it invisibly.
// Those are reported by ValidateConfig instead, which names each offending
// binding so a human can decide. That is why `rela migrate` is not always
// sufficient to make a project load: it is the easy-mode upgrade for the cases
// that have one right answer, not a rubber stamp over the ones that do not.
type RelationDirectionMigration struct {
	meta MetamodelProvider
}

// SetMetamodel implements MetamodelAware.
func (m *RelationDirectionMigration) SetMetamodel(meta MetamodelProvider) {
	m.meta = meta
}

func (m *RelationDirectionMigration) Name() string {
	return "relation-direction"
}

func (m *RelationDirectionMigration) Description() string {
	return "Write explicit direction: on relation bindings that omit it"
}

func (m *RelationDirectionMigration) FileTypes() []FileType {
	return []FileType{FileTypeDataEntry}
}

func (m *RelationDirectionMigration) Detect(doc *yaml.Node) bool {
	return len(m.bindings(doc)) > 0
}

func (m *RelationDirectionMigration) Apply(doc *yaml.Node) error {
	for _, e := range m.bindings(doc) {
		SetMapValue(e.node, "direction", e.direction)
	}
	return nil
}

// bindings returns every relation binding in the document that is missing a
// `direction:` and can have one inferred, paired with the direction to write.
//
// Detect and Apply share this one traversal so they cannot disagree about what
// counts as migratable — the earlier apply-bool variant made that agreement a
// property of four nested early-returns rather than of the structure.
func (m *RelationDirectionMigration) bindings(doc *yaml.Node) []directionEdit {
	if m.meta == nil {
		return nil // without a metamodel nothing can be inferred
	}
	root := GetDocumentRoot(doc)
	if root == nil {
		return nil
	}
	edits := make([]directionEdit, 0, 8)
	edits = append(edits, m.formBindings(root)...)
	edits = append(edits, m.listBindings(root)...)
	edits = append(edits, m.kanbanBindings(root)...)
	edits = append(edits, m.caldavBindings(root)...)
	return edits
}

// directionEdit is one binding to rewrite: the mapping node and the direction
// value to set on it.
type directionEdit struct {
	node      *yaml.Node
	direction string
}

// formBindings covers `forms.<id>.relations` plus each wizard step's own list.
func (m *RelationDirectionMigration) formBindings(root *yaml.Node) []directionEdit {
	var edits []directionEdit
	forEachTypedSection(root, "forms", func(formDef *yaml.Node, entityType string) {
		edits = append(edits, m.seqBindings(GetMapValue(formDef, "relations"), entityType)...)
		steps := GetMapValue(formDef, "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			return
		}
		for _, step := range steps.Content {
			if step.Kind == yaml.MappingNode {
				edits = append(edits, m.seqBindings(GetMapValue(step, "relations"), entityType)...)
			}
		}
	})
	return edits
}

// listBindings covers `lists.<id>.columns` and `lists.<id>.filter_controls`.
func (m *RelationDirectionMigration) listBindings(root *yaml.Node) []directionEdit {
	var edits []directionEdit
	forEachTypedSection(root, "lists", func(listDef *yaml.Node, entityType string) {
		edits = append(edits, m.seqBindings(GetMapValue(listDef, "columns"), entityType)...)
		edits = append(edits, m.seqBindings(GetMapValue(listDef, "filter_controls"), entityType)...)
	})
	return edits
}

// kanbanBindings covers `kanbans.<id>.card.fields` and its filter_controls.
func (m *RelationDirectionMigration) kanbanBindings(root *yaml.Node) []directionEdit {
	var edits []directionEdit
	forEachTypedSection(root, "kanbans", func(kanbanDef *yaml.Node, entityType string) {
		if card := GetMapValue(kanbanDef, "card"); card != nil && card.Kind == yaml.MappingNode {
			edits = append(edits, m.seqBindings(GetMapValue(card, "fields"), entityType)...)
		}
		edits = append(edits, m.seqBindings(GetMapValue(kanbanDef, "filter_controls"), entityType)...)
	})
	return edits
}

// caldavBindings covers `caldav.dynamic.<name>`, whose edge runs member→driver
// — so the MEMBER type (`entity_type`) anchors the inference, not driver_type.
// Each entry is itself the binding, rather than an element of a sequence.
func (m *RelationDirectionMigration) caldavBindings(root *yaml.Node) []directionEdit {
	caldav := GetMapValue(root, "caldav")
	if caldav == nil || caldav.Kind != yaml.MappingNode {
		return nil
	}
	dynamic := GetMapValue(caldav, "dynamic")
	if dynamic == nil || dynamic.Kind != yaml.MappingNode {
		return nil
	}
	var edits []directionEdit
	for i := 1; i < len(dynamic.Content); i += 2 {
		coll := dynamic.Content[i]
		if coll.Kind != yaml.MappingNode {
			continue
		}
		if dir, ok := m.inferDirection(coll, getScalarValue(coll, "entity_type")); ok {
			edits = append(edits, directionEdit{node: coll, direction: dir})
		}
	}
	return edits
}

// seqBindings collects the migratable bindings in one sequence of mappings.
func (m *RelationDirectionMigration) seqBindings(seq *yaml.Node, entityType string) []directionEdit {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil
	}
	var edits []directionEdit
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		if dir, ok := m.inferDirection(item, entityType); ok {
			edits = append(edits, directionEdit{node: item, direction: dir})
		}
	}
	return edits
}

// forEachTypedSection visits each entry of a top-level map whose values carry
// an `entity_type` (forms, lists, kanbans all share that shape).
func forEachTypedSection(root *yaml.Node, key string, fn func(def *yaml.Node, entityType string)) {
	section := GetMapValue(root, key)
	if section == nil || section.Kind != yaml.MappingNode {
		return
	}
	for i := 1; i < len(section.Content); i += 2 {
		def := section.Content[i]
		if def.Kind != yaml.MappingNode {
			continue
		}
		if entityType := getScalarValue(def, "entity_type"); entityType != "" {
			fn(def, entityType)
		}
	}
}

// inferDirection returns the direction to write for a binding, and whether it
// should be written at all.
//
// This mirrors dataentryconfig.InferDirection and must stay in lockstep with
// it. It cannot call that function directly: migrations walk yaml.Node against
// the narrow MetamodelProvider interface, not a *metamodel.Metamodel. It reports false when the binding already has a
// direction, names an unknown relation, or is ambiguous (entity type on both
// sides) — the last of which is left for the author to resolve.
func (m *RelationDirectionMigration) inferDirection(rel *yaml.Node, entityType string) (string, bool) {
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
