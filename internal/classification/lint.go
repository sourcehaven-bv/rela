package classification

import "fmt"

// Lint checks a parsed file against the schema: every field has an entry,
// no entry is still needs-review, no entry or override names something the
// schema does not have. It returns only schema issues; combine it with the
// issues from [Parse].
func Lint(f *File, shape Shape) []Issue {
	var issues []Issue //nolint:prealloc // three appends of unknown length
	entityFields := make(map[string][]string, len(shape.Entities))
	for name, t := range shape.Entities {
		entityFields[name] = t.Fields
	}
	relationFields := make(map[string][]string, len(shape.Relations))
	for name, r := range shape.Relations {
		relationFields[name] = r.Fields
	}
	issues = append(issues, lintSection("assign", "entity type", f.Assign, entityFields)...)
	issues = append(issues, lintSection("assign_relations", "relation type", f.AssignRelations, relationFields)...)
	issues = append(issues, lintOverrides(f.Overrides, shape)...)
	issues = append(issues, f.lintHops(shape)...)
	return issues
}

// lintHops reports a subject_hops entry on a relation that is not a subject
// link: traversal never follows it, so the entry does nothing.
func (f *File) lintHops(shape Shape) []Issue {
	subjects := map[string]bool{}
	for _, s := range f.Subjects(shape) {
		subjects[s.Type] = true
	}
	var issues []Issue
	for _, name := range sortedKeys(f.Overrides.SubjectHops) {
		r, ok := shape.Relations[name]
		if !ok || f.isSubjectLink(name, r, subjects) {
			continue
		}
		path := "subject_hops." + name
		issues = append(issues, Issue{
			Code: CodeNoEffect, Path: path, Line: f.Overrides.Lines[path],
			Message: fmt.Sprintf("relation %q is not a subject link, so it is never followed; "+
				"add `subject_link: {%s: true}` or remove the entry", name, name),
		})
	}
	return issues
}

func lintSection(section, kind string, assigned map[string]TypeAssignments, schema map[string][]string) []Issue {
	var issues []Issue
	for _, typeName := range sortedKeys(schema) {
		fields := schema[typeName]
		if len(fields) == 0 {
			continue
		}
		ta, ok := assigned[typeName]
		if !ok {
			issues = append(issues, Issue{
				Code: CodeMissingEntry, Path: section + "." + typeName,
				Message: fmt.Sprintf("%s %q has %d field(s) without an entry; run `rela classification sync`",
					kind, typeName, len(fields)),
			})
			continue
		}
		for _, field := range fields {
			path := section + "." + typeName + "." + field
			a, ok := ta.Fields[field]
			switch {
			case !ok:
				issues = append(issues, Issue{
					Code: CodeMissingEntry, Path: path, Line: ta.Line,
					Message: "field has no entry; add labels, none, or run `rela classification sync`",
				})
			case a.State == NeedsReview:
				issues = append(issues, Issue{
					Code: CodeNeedsReview, Path: path, Line: a.Line,
					Message: "field is not reviewed; replace needs-review with labels or none",
				})
			}
		}
	}
	for _, typeName := range sortedKeys(assigned) {
		ta := assigned[typeName]
		fields, ok := schema[typeName]
		if !ok {
			issues = append(issues, Issue{
				Code: CodeStaleEntry, Path: section + "." + typeName, Line: ta.Line,
				Message: fmt.Sprintf("the schema has no %s %q; remove the entry or run `rela classification sync` "+
					"after a rename", kind, typeName),
			})
			continue
		}
		known := make(map[string]bool, len(fields))
		for _, field := range fields {
			known[field] = true
		}
		for _, field := range sortedKeys(ta.Fields) {
			if !known[field] {
				issues = append(issues, Issue{
					Code: CodeStaleEntry, Path: section + "." + typeName + "." + field, Line: ta.Fields[field].Line,
					Message: fmt.Sprintf("%s %q has no field %q", kind, typeName, field),
				})
			}
		}
	}
	return issues
}

func lintOverrides(o Overrides, shape Shape) []Issue {
	var issues []Issue
	check := func(section string, names []string, exists func(string) bool, kind string) {
		for _, name := range names {
			if !exists(name) {
				path := section + "." + name
				issues = append(issues, Issue{
					Code: CodeUnknownTarget, Path: path, Line: o.Lines[path],
					Message: fmt.Sprintf("the schema has no %s %q", kind, name),
				})
			}
		}
	}
	hasEntity := func(n string) bool { _, ok := shape.Entities[n]; return ok }
	hasRelation := func(n string) bool { _, ok := shape.Relations[n]; return ok }
	check("subject", sortedKeys(o.Subject), hasEntity, "entity type")
	check("subject_link", sortedKeys(o.SubjectLink), hasRelation, "relation type")
	check("subject_hops", sortedKeys(o.SubjectHops), hasRelation, "relation type")
	return issues
}
