package classification

import (
	"fmt"
	"slices"
)

// MaxProfileTypes caps the entity types in one subject profile. A hub type
// reachable from everywhere would otherwise make every profile the whole
// schema; the profile is marked truncated instead.
const MaxProfileTypes = 64

// Subject is an entity type that represents a person, and why rela thinks so.
type Subject struct {
	Type string `json:"type"`
	// Reason is the provenance: the field and label that made the type a
	// subject, or "subject override".
	Reason string `json:"reason"`
}

// Hop is one step from a subject to a type whose data is about it.
type Hop struct {
	Relation string `json:"relation"`
	// Outgoing is true when the relation points away from the type the hop
	// starts at.
	Outgoing bool   `json:"outgoing"`
	Type     string `json:"type"`
}

// Reach is an entity type in a subject's profile and the path to it.
type Reach struct {
	Type string `json:"type"`
	Path []Hop  `json:"path"`
}

// Profile is the data about one subject type: the type itself and the types
// its subject links reach.
type Profile struct {
	Subject string  `json:"subject"`
	Reached []Reach `json:"reached"`
	// Relations lists the relation types traversed, whose own fields are part
	// of the profile too.
	Relations []string `json:"relations"`
	Truncated bool     `json:"truncated,omitempty"`
}

// ownFields returns an entity type's labeled fields, with Ref "type.field".
func (f *File) ownFields(typeName string, t TypeShape) []Field {
	return f.labeledFields(typeName, f.Assign[typeName], t.Fields)
}

func (f *File) relationFields(name string, r RelationShape) []Field {
	return f.labeledFields("~"+name, f.AssignRelations[name], r.Fields)
}

func (f *File) labeledFields(prefix string, ta TypeAssignments, fields []string) []Field {
	var out []Field
	for _, name := range fields {
		a, ok := ta.Fields[name]
		if !ok || a.State != Labeled {
			continue
		}
		out = append(out, Field{Ref: prefix + "." + name, Labels: a.Labels})
	}
	return out
}

// Subjects returns the entity types that represent a person, sorted. A type
// is a subject when one of its own fields carries a label with the
// direct-identifier role, directly or through a record-scope derived label.
// The subject override wins either way.
func (f *File) Subjects(shape Shape) []Subject {
	var out []Subject
	for _, name := range sortedKeys(shape.Entities) {
		if forced, ok := f.Overrides.Subject[name]; ok {
			if forced {
				out = append(out, Subject{Type: name, Reason: "subject override"})
			}
			continue
		}
		if reason := f.identifies(name, shape.Entities[name]); reason != "" {
			out = append(out, Subject{Type: name, Reason: reason})
		}
	}
	return out
}

// identifies returns why the type's own fields identify a person, or "".
func (f *File) identifies(name string, t TypeShape) string {
	fields := f.ownFields(name, t)
	for _, field := range fields {
		for _, label := range field.Labels {
			if f.roleOf(label) == RoleDirectIdentifier {
				return fmt.Sprintf("%s has %s (direct-identifier)", field.Ref, label)
			}
		}
	}
	for _, label := range f.Derive(ScopeRecord, fields) {
		if f.roleOf(label) == RoleDirectIdentifier {
			return fmt.Sprintf("derived %s (direct-identifier) from %s", label, joinRefs(f.contributors(label, fields)))
		}
	}
	return ""
}

// isSubjectLink reports whether traversal may follow relation name. By
// default a relation is a subject link when one of its ends is a subject
// type; the subject_link override wins either way.
func (f *File) isSubjectLink(name string, r RelationShape, subjects map[string]bool) bool {
	if forced, ok := f.Overrides.SubjectLink[name]; ok {
		return forced
	}
	for _, e := range r.Ends {
		if subjects[e.From] || subjects[e.To] {
			return true
		}
	}
	return false
}

// roleOf returns label's role, or "" for a label the file does not define
// (a file with lint issues can still be reported on).
func (f *File) roleOf(label string) Role {
	if l := f.Labels[label]; l != nil {
		return l.Role
	}
	return ""
}

func (f *File) hops(relation string) int {
	if h, ok := f.Overrides.SubjectHops[relation]; ok {
		return h
	}
	return 1
}

// Profiles returns the profile of every subject type, sorted by subject.
//
// Traversal is breadth-first from the subject. A relation may be followed at
// distance d (0 at the subject) when it is a subject link and d < its hop
// limit. Traversal never enters another subject type, so a relation between
// two people (including a self-relation) never merges their data.
func (f *File) Profiles(shape Shape) []Profile {
	subjects := map[string]bool{}
	for _, s := range f.Subjects(shape) {
		subjects[s.Type] = true
	}
	links := map[string]RelationShape{}
	for name, r := range shape.Relations {
		if f.isSubjectLink(name, r, subjects) {
			links[name] = r
		}
	}

	w := &walker{f: f, subjects: subjects, links: links, linkNames: sortedKeys(links)}
	out := make([]Profile, 0, len(subjects))
	for _, subject := range sortedKeys(subjects) {
		out = append(out, w.profile(subject))
	}
	return out
}

// walker holds what every profile traversal shares.
type walker struct {
	f         *File
	subjects  map[string]bool
	links     map[string]RelationShape
	linkNames []string
}

func (w *walker) profile(subject string) Profile {
	p := Profile{Subject: subject}
	paths := map[string][]Hop{subject: nil}
	relations := map[string]bool{}
	frontier := []string{subject}
	for depth := 0; len(frontier) > 0 && depth < MaxSubjectHops; depth++ {
		var next []string
		for _, from := range frontier {
			for _, hop := range w.hopsFrom(from, depth) {
				if _, seen := paths[hop.Type]; seen {
					relations[hop.Relation] = true
					continue
				}
				if len(paths) >= MaxProfileTypes {
					p.Truncated = true
					continue
				}
				relations[hop.Relation] = true
				paths[hop.Type] = append(slices.Clone(paths[from]), hop)
				next = append(next, hop.Type)
			}
		}
		frontier = next
	}
	for _, t := range sortedKeys(paths) {
		if t != subject {
			p.Reached = append(p.Reached, Reach{Type: t, Path: paths[t]})
		}
	}
	p.Relations = sortedKeys(relations)
	return p
}

// hopsFrom returns the hops allowed from type from at distance depth: along
// subject links whose hop limit exceeds depth, never into a subject type.
func (w *walker) hopsFrom(from string, depth int) []Hop {
	var out []Hop
	for _, rel := range w.linkNames {
		if depth >= w.f.hops(rel) {
			continue
		}
		for _, hop := range steps(rel, w.links[rel], from) {
			if !w.subjects[hop.Type] {
				out = append(out, hop)
			}
		}
	}
	return out
}

// steps returns the hops relation r allows from type from, in either
// direction, deduplicated and sorted.
func steps(name string, r RelationShape, from string) []Hop {
	var out []Hop
	for _, e := range r.Ends {
		if e.From == from {
			out = append(out, Hop{Relation: name, Outgoing: true, Type: e.To})
		}
		if e.To == from {
			out = append(out, Hop{Relation: name, Outgoing: false, Type: e.From})
		}
	}
	slices.SortFunc(out, func(a, b Hop) int {
		if a.Type != b.Type {
			if a.Type < b.Type {
				return -1
			}
			return 1
		}
		switch {
		case a.Outgoing == b.Outgoing:
			return 0
		case a.Outgoing:
			return -1
		default:
			return 1
		}
	})
	return slices.Compact(out)
}

// ProfileFields returns the labeled fields in a profile: the subject's, each
// reached type's, and each traversed relation's. Each record's record-scope
// derived labels come along as pseudo-fields, so a subject-scope rule can
// build on them.
func (f *File) ProfileFields(p Profile, shape Shape) []Field {
	var fields []Field
	add := func(prefix string, own []Field) {
		fields = append(fields, own...)
		for _, d := range f.Derivations(ScopeRecord, own) {
			fields = append(fields, Field{
				Ref: prefix + ".@" + d.Label, Labels: []string{d.Label}, Derived: true, Sources: d.Fields,
			})
		}
	}
	add(p.Subject, f.ownFields(p.Subject, shape.Entities[p.Subject]))
	for _, r := range p.Reached {
		add(r.Type, f.ownFields(r.Type, shape.Entities[r.Type]))
	}
	for _, rel := range p.Relations {
		add("~"+rel, f.relationFields(rel, shape.Relations[rel]))
	}
	return fields
}
