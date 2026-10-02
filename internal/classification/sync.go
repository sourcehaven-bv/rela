package classification

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// RenameKind says what a schema rename renamed.
type RenameKind int

// The rename kinds data-migration files record.
const (
	RenameProperty RenameKind = iota
	RenameEntityType
	RenameRelationType
)

// Rename is one rename step from the data-migration history. Owner is the
// entity type a property rename applies to.
type Rename struct {
	Kind            RenameKind
	Owner, From, To string
}

// SyncResult reports what [Sync] changed and what needs a human.
type SyncResult struct {
	// Output is the new file content; equal to the input when Changed is false.
	Output  []byte
	Changed bool
	// Added lists entries written as needs-review.
	Added []string
	// Moved lists entries carried across a rename, as "old -> new".
	Moved []string
	// Stale lists entries the schema no longer has. Sync never deletes them.
	Stale []string
	// Conflicts lists renames sync could not apply because the new name
	// already has an entry.
	Conflicts []string
}

// Sync brings the file in line with the schema: it moves entries across
// renames recorded in renames (chronological order), adds every field without
// an entry as needs-review, and reports entries the schema no longer has. It
// never deletes an entry or changes a label.
//
// data may be empty (no file yet). Sync refuses a file that does not parse
// cleanly, because editing a file it only partly understood could drop what
// it did not understand. Comments and key order are kept; blank lines and
// custom indentation are not (the YAML encoder rewrites layout).
func Sync(data []byte, shape Shape, renames []Rename) (SyncResult, []Issue) {
	if _, issues := Parse(data); len(issues) > 0 {
		return SyncResult{Output: data}, issues
	}
	doc, err := documentFor(data)
	if err != nil {
		return SyncResult{Output: data}, []Issue{{Code: CodeSyntax, Message: err.Error()}}
	}
	root := doc.Content[0]
	s := &syncer{shape: shape, renames: renames}

	entityFields := map[string][]string{}
	for name, t := range shape.Entities {
		entityFields[name] = t.Fields
	}
	relationFields := map[string][]string{}
	for name, r := range shape.Relations {
		relationFields[name] = r.Fields
	}
	s.section(root, "assign", entityFields, RenameEntityType)
	s.section(root, "assign_relations", relationFields, RenameRelationType)
	s.overrideKeys(root, "subject", RenameEntityType)
	s.overrideKeys(root, "subject_link", RenameRelationType)
	s.overrideKeys(root, "subject_hops", RenameRelationType)

	res := s.result
	if !s.changed {
		res.Output = data
		return res, nil
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return SyncResult{Output: data}, []Issue{{Code: CodeSyntax, Message: err.Error()}}
	}
	if err := enc.Close(); err != nil {
		return SyncResult{Output: data}, []Issue{{Code: CodeSyntax, Message: err.Error()}}
	}
	res.Output, res.Changed = buf.Bytes(), true
	return res, nil
}

// documentFor parses data, or starts a new document when data is empty.
func documentFor(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 && doc.Content[0].Kind == yaml.MappingNode {
		return &doc, nil
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	labels := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Style: yaml.FlowStyle}
	key := scalarNode("labels")
	key.HeadComment = "# Data classification: describes what data each field holds. It changes no\n" +
		"# behavior and is not access control. See docs/classification.md."
	// yaml.v3 drops the comments of a document with no content, so a file
	// holding only comments has them carried over from the raw text.
	if kept := commentLines(data); kept != "" {
		key.HeadComment = kept + "\n\n" + key.HeadComment
	}
	root.Content = append(root.Content, key, labels)
	return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}, nil
}

// commentLines returns the lines of data that are YAML comments, joined.
func commentLines(data []byte) string {
	var out []string
	for line := range strings.Lines(string(data)) {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

type syncer struct {
	shape   Shape
	renames []Rename
	changed bool
	result  SyncResult
}

// resolveType follows type renames of kind from name, in history order.
func (s *syncer) resolveType(name string, kind RenameKind) string {
	for _, r := range s.renames {
		if r.Kind == kind && r.From == name {
			name = r.To
		}
	}
	return name
}

// resolveProperty follows a property through type and property renames, in
// history order: a type rename changes the owner later property steps name.
func (s *syncer) resolveProperty(typeName, prop string) (owner, name string) {
	for _, r := range s.renames {
		switch {
		case r.Kind == RenameEntityType && r.From == typeName:
			typeName = r.To
		case r.Kind == RenameProperty && r.Owner == typeName && r.From == prop:
			prop = r.To
		}
	}
	return typeName, prop
}

// historicalNames returns typeName and every earlier entity type name that
// renames into it.
func (s *syncer) historicalNames(typeName string) []string {
	names := []string{typeName}
	for _, r := range s.renames {
		if r.Kind == RenameEntityType && r.From != typeName && s.resolveType(r.From, RenameEntityType) == typeName {
			names = append(names, r.From)
		}
	}
	return names
}

// section syncs `assign` or `assign_relations`.
func (s *syncer) section(root *yaml.Node, section string, schema map[string][]string, typeRename RenameKind) {
	sec := mapValue(root, section)
	if sec == nil {
		if !anyFields(schema) {
			return
		}
		sec = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, scalarNode(section), sec)
		s.changed = true
	} else if sec.Kind != yaml.MappingNode { // null value
		*sec = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}

	// Type keys the schema no longer has: carry them across a rename, or
	// report them. Only a key whose name is gone from the schema is moved,
	// so a new type that reuses an old name keeps its own entry.
	for _, key := range mapKeys(sec) {
		if _, ok := schema[key.Value]; ok {
			continue
		}
		target := s.resolveType(key.Value, typeRename)
		_, targetInSchema := schema[target]
		switch {
		case target == key.Value || !targetInSchema:
			s.result.Stale = append(s.result.Stale, section+"."+key.Value)
		case mapValue(sec, target) != nil:
			s.result.Conflicts = append(s.result.Conflicts,
				fmt.Sprintf("%s.%s -> %s: %s already has an entry", section, key.Value, target, target))
		default:
			s.result.Moved = append(s.result.Moved, fmt.Sprintf("%s.%s -> %s", section, key.Value, target))
			key.Value = target
			s.changed = true
		}
	}

	for _, typeName := range sortedKeys(schema) {
		fields := schema[typeName]
		if len(fields) == 0 {
			continue
		}
		typeNode := mapValue(sec, typeName)
		if typeNode == nil {
			typeNode = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			sec.Content = append(sec.Content, scalarNode(typeName), typeNode)
			s.changed = true
		} else if typeNode.Kind != yaml.MappingNode { // null value
			*typeNode = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		s.fields(typeNode, section, typeName, fields, typeRename == RenameEntityType)
	}
}

func (s *syncer) fields(typeNode *yaml.Node, section, typeName string, fields []string, entity bool) {
	known := make(map[string]bool, len(fields))
	for _, f := range fields {
		known[f] = true
	}
	for _, key := range mapKeys(typeNode) {
		if known[key.Value] {
			continue
		}
		target := key.Value
		if entity {
			// The entry may date from any earlier name of this type, so try
			// the property's history under each of them.
			for _, owner := range s.historicalNames(typeName) {
				if _, p := s.resolveProperty(owner, key.Value); p != key.Value {
					target = p
					break
				}
			}
		}
		path := section + "." + typeName + "." + key.Value
		switch {
		case target == key.Value || !known[target]:
			s.result.Stale = append(s.result.Stale, path)
		case mapValue(typeNode, target) != nil:
			s.result.Conflicts = append(s.result.Conflicts,
				fmt.Sprintf("%s -> %s: %s already has an entry", path, target, target))
		default:
			s.result.Moved = append(s.result.Moved, path+" -> "+target)
			key.Value = target
			s.changed = true
		}
	}

	prev := ""
	for _, field := range fields {
		if mapValue(typeNode, field) != nil {
			prev = field
			continue
		}
		insertAfter(typeNode, prev, scalarNode(field), scalarNode(StateNeedsReview))
		s.result.Added = append(s.result.Added, section+"."+typeName+"."+field)
		s.changed = true
		prev = field
	}
}

// overrideKeys carries override keys across type or relation renames.
func (s *syncer) overrideKeys(root *yaml.Node, section string, kind RenameKind) {
	sec := mapValue(root, section)
	if sec == nil || sec.Kind != yaml.MappingNode {
		return
	}
	var exists func(string) bool
	if kind == RenameEntityType {
		exists = func(n string) bool { _, ok := s.shape.Entities[n]; return ok }
	} else {
		exists = func(n string) bool { _, ok := s.shape.Relations[n]; return ok }
	}
	for _, key := range mapKeys(sec) {
		if exists(key.Value) {
			continue
		}
		target := s.resolveType(key.Value, kind)
		switch {
		case target == key.Value || !exists(target):
			s.result.Stale = append(s.result.Stale, section+"."+key.Value)
		case mapValue(sec, target) != nil:
			s.result.Conflicts = append(s.result.Conflicts,
				fmt.Sprintf("%s.%s -> %s: %s already has an entry", section, key.Value, target, target))
		default:
			s.result.Moved = append(s.result.Moved, fmt.Sprintf("%s.%s -> %s", section, key.Value, target))
			key.Value = target
			s.changed = true
		}
	}
}

func anyFields(schema map[string][]string) bool {
	for _, f := range schema {
		if len(f) > 0 {
			return true
		}
	}
	return false
}

// ---- yaml.Node helpers (kept local so this package stays a leaf) ----

func scalarNode(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

// mapValue returns the value for key in mapping m, or nil.
func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// mapKeys returns the key nodes of mapping m. Editing a key node's Value
// renames the key in place, keeping its position and comments.
func mapKeys(m *yaml.Node) []*yaml.Node {
	keys := make([]*yaml.Node, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		keys = append(keys, m.Content[i])
	}
	return keys
}

// insertAfter inserts key/value into mapping m right after the key named
// after, or first when after is empty or absent.
func insertAfter(m *yaml.Node, after string, key, value *yaml.Node) {
	at := 0
	if after != "" {
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value == after {
				at = i + 2
				break
			}
		}
	}
	m.Content = slices.Insert(m.Content, at, key, value)
}
