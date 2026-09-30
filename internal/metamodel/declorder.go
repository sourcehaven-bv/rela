package metamodel

import (
	"fmt"
	"slices"
	"sort"

	"gopkg.in/yaml.v3"
)

// Declaration order of faces and worlds (TKT-7IZHP0 design §3.1).
//
// EntityDef.Faces and Metamodel.Worlds are maps, so YAML order is lost when
// they unmarshal. The order is recorded beside them at load, from the same
// yaml.Node pass that records PropertyOrder, and read through [FaceOrderOf]
// and [WorldOrderOf]. Both are free functions rather than methods because
// EntityDef and Metamodel are at their plimsoll exported-method lines.
//
// A metamodel built in Go, without YAML, records no order; the accessors
// then fall back to sorted names. A LOADED metamodel must record one for
// every faced type and for its worlds: [validateDeclOrder] fails the load
// otherwise, so the fallback cannot silently stand in for a YAML order that
// an included file failed to report.

// FaceOrderOf returns entityType's face names in declaration order, as a
// copy. Without a recorded order it returns the sorted names; a faceless or
// unknown type returns nil. An alias resolves to its canonical type.
//
// It reads the two fields it needs through the map rather than taking an
// EntityDef, which would copy the whole definition on every call (RR-TLQPK6).
// Nil: accepted, returns nil.
func FaceOrderOf(m *Metamodel, entityType string) []string {
	if m == nil {
		return nil
	}
	if _, ok := m.Entities[entityType]; !ok {
		entityType = m.ResolveAlias(entityType)
	}
	if order := m.Entities[entityType].faceOrder; order != nil {
		return slices.Clone(order)
	}
	return sortedNames(m.Entities[entityType].Faces)
}

// WorldOrderOf returns m's declared world names in declaration order, as a
// copy, not including the implicit default world. Without a recorded order
// it returns the sorted names.
// Nil: accepted, returns nil.
func WorldOrderOf(m *Metamodel) []string {
	if m == nil {
		return nil
	}
	if m.worldOrder != nil {
		return slices.Clone(m.worldOrder)
	}
	return sortedNames(m.Worlds)
}

func sortedNames[V any](m map[string]V) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// resolveAlias follows an alias node to the node it names.
func resolveAlias(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

// mappingEntry is one key of a mapping as the decoder sees it.
type mappingEntry struct {
	key string
	val *yaml.Node
}

// mappingEntries returns the entries of a mapping node the way yaml.v3
// decodes them into a map: aliases are followed and `<<` merge keys are
// expanded in place. A key is listed once, at its first position in the
// document. Its value is the explicit one when the mapping sets the key
// itself, else the first merged one, which is the decoder's precedence.
// A node that is not a mapping has no entries.
func mappingEntries(n *yaml.Node) []mappingEntry {
	n = resolveAlias(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var out []mappingEntry
	index := make(map[string]int)
	explicit := make(map[string]bool)
	add := func(key string, val *yaml.Node, isExplicit bool) {
		i, seen := index[key]
		switch {
		case !seen:
			index[key] = len(out)
			out = append(out, mappingEntry{key: key, val: val})
		case isExplicit && !explicit[key]:
			out[i].val = val
		}
		if isExplicit {
			explicit[key] = true
		}
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		if key.ShortTag() != "!!merge" {
			add(key.Value, val, true)
			continue
		}
		sources := []*yaml.Node{val}
		if v := resolveAlias(val); v != nil && v.Kind == yaml.SequenceNode {
			sources = v.Content
		}
		for _, src := range sources {
			for _, e := range mappingEntries(src) {
				add(e.key, e.val, false)
			}
		}
	}
	return out
}

// mappingKeys returns the keys of a mapping node in document order, with
// aliases followed and merge keys expanded (see mappingEntries).
func mappingKeys(n *yaml.Node) []string {
	entries := mappingEntries(n)
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		keys = append(keys, e.key)
	}
	return keys
}

// mappingValue returns the value node under key in a mapping node, with the
// alias followed.
func mappingValue(n *yaml.Node, key string) (*yaml.Node, bool) {
	for _, e := range mappingEntries(n) {
		if e.key == key {
			return resolveAlias(e.val), true
		}
	}
	return nil, false
}

// documentMapping returns the top-level mapping of a parsed YAML document.
func documentMapping(root *yaml.Node) (*yaml.Node, bool) {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, false
	}
	doc := resolveAlias(root.Content[0])
	return doc, doc != nil && doc.Kind == yaml.MappingNode
}

// recordFaceOrder stores, on every entity in entities, the order of the
// `faces:` keys its definition declares in the `entities:` node. Anchors,
// aliases and merge keys resolve as the decoder resolves them, so the order
// names exactly the faces that were decoded.
func recordFaceOrder(entitiesNode *yaml.Node, entities map[string]EntityDef) {
	for _, e := range mappingEntries(entitiesNode) {
		def, ok := entities[e.key]
		if !ok {
			continue
		}
		faces, ok := mappingValue(e.val, "faces")
		if !ok || faces.Kind != yaml.MappingNode {
			continue
		}
		def.faceOrder = mappingKeys(faces)
		entities[e.key] = def
	}
}

// recordFileFaceOrder parses data once more as a node tree and records the
// face order of the entities it declares. It is how an included file's order
// reaches the merged metamodel (design A7).
func recordFileFaceOrder(data []byte, entities map[string]EntityDef) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil { // coverage-ignore: defensive: the same bytes already
		// unmarshaled into a struct; a node parse is strictly more permissive
		return fmt.Errorf("parse yaml.Node for face order: %w", err)
	}
	doc, ok := documentMapping(&root)
	if !ok {
		return nil
	}
	if entitiesNode, ok := mappingValue(doc, "entities"); ok {
		recordFaceOrder(entitiesNode, entities)
	}
	return nil
}

// validateDeclOrder asserts that each recorded order is a permutation of its
// map's keys, and that a loaded metamodel recorded one for every faced type
// and for its worlds. It runs only on YAML-loaded metamodels, where a missing
// order means a loader path forgot to record it.
func validateDeclOrder(m *Metamodel) []string {
	var errs []string
	for _, name := range sortedKeys(m.Entities) {
		def := m.Entities[name]
		if len(def.Faces) == 0 {
			continue
		}
		if def.faceOrder == nil {
			errs = append(errs, fmt.Sprintf(
				"entity %q: internal: the declaration order of its faces was not recorded at load", name))
			continue
		}
		if !isPermutation(def.faceOrder, def.Faces) {
			errs = append(errs, fmt.Sprintf(
				"entity %q: internal: recorded face order %v does not match its declared faces", name, def.faceOrder))
		}
	}
	if len(m.Worlds) > 0 {
		switch {
		case m.worldOrder == nil:
			errs = append(errs, "worlds: internal: the declaration order of the worlds was not recorded at load")
		case !isPermutation(m.worldOrder, m.Worlds):
			errs = append(errs, fmt.Sprintf(
				"worlds: internal: recorded world order %v does not match the declared worlds", m.worldOrder))
		}
	}
	return errs
}

// isPermutation reports whether order names every key of m exactly once.
func isPermutation[V any](order []string, m map[string]V) bool {
	if len(order) != len(m) {
		return false
	}
	seen := make(map[string]bool, len(order))
	for _, k := range order {
		if _, ok := m[k]; !ok || seen[k] {
			return false
		}
		seen[k] = true
	}
	return true
}

// validateDefaultWorld checks the top-level `default_world:` key: the world a
// request uses when it names none (TKT-7IZHP0 design §21 D3). With worlds
// declared it must name one of them; with none it may only be the generated
// default world. Unset is always valid. PR 1 validates the key; the worlds
// compiler starts reading it in PR 5a.
func validateDefaultWorld(m *Metamodel) []string {
	w := m.DefaultWorld
	if w == "" {
		return nil
	}
	if len(m.Worlds) == 0 {
		if w == DefaultWorldName {
			return nil
		}
		return []string{fmt.Sprintf(
			"default_world: %q is not a world; no worlds are declared, so the only world is the generated %q",
			w, DefaultWorldName)}
	}
	if _, ok := m.Worlds[w]; ok {
		return nil
	}
	return []string{fmt.Sprintf(
		"default_world: %q is not a declared world (declared, in order: %v)", w, WorldOrderOf(m))}
}
