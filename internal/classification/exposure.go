package classification

import (
	"slices"
	"strings"
)

// Caps on what one view's analysis enumerates.
const (
	MaxBreakers        = 20
	MaxFindingsPerView = 200
	// MaxRemovals is the largest set of fields searched for when no single
	// removal breaks a combination.
	MaxRemovals = 3
	// maxRemovalFields caps the contributors that search runs over, since it
	// tries every subset up to MaxRemovals.
	maxRemovalFields = 24
)

// The exposure rules, named like `rela acl audit` rules.
const (
	RuleReadsLabel     = "C1-role-reads-label"
	RuleDerivedRecord  = "C2-derived-record"
	RuleDerivedSubject = "C3-derived-subject"
	subjectPrefix      = "subject "
)

// View is what one principal could read, worst case: every conditional
// grant counted as granted. The CLI computes it through the ACL runtime, so
// this package never evaluates a policy.
type View struct {
	// Entities maps each readable entity type to its visible fields.
	Entities map[string][]string
	// Relations maps each relation type with at least one readable
	// (from, to) pair to its visible fields.
	Relations map[string][]string
	// Ends lists, per relation type, the (from, to) pairs whose both
	// endpoints are readable.
	Ends map[string][]Ends
}

// Exposure is one thing a view can read that the classification marks.
type Exposure struct {
	// Rule is C1-role-reads-label, C2-derived-record or C3-derived-subject.
	Rule string `json:"rule"`
	// Subject is the entity type, "~relation", or "subject <type>".
	Subject string   `json:"subject"`
	Label   string   `json:"label"`
	Fields  []string `json:"fields"`
	// Breakers are fields whose removal from the view alone would stop a
	// derived label from holding. Empty when no single removal suffices.
	Breakers          []string `json:"breakers,omitempty"`
	BreakersTruncated bool     `json:"breakers_truncated,omitempty"`
	// MinRemovals is the fewest fields whose removal together stops the
	// derived label from holding: 1 when Breakers is set, up to
	// [MaxRemovals], and 0 when more are needed or the search was skipped
	// because the label has more than 24 contributing fields.
	MinRemovals int `json:"min_removals,omitempty"`
}

// Exposures returns what view v can read that f labels: C1, then C2, then
// C3 findings, each group by entity type, then relation, then subject.
// truncated is set when more than [MaxFindingsPerView] were found. The cut
// keeps combinations first (C3, then C2), because they are the findings an
// operator is least likely to see for themselves.
//
// It never says the view is wrong. Whether a role should read a label is the
// operator's decision; this lists what it does read, so the decision is made
// knowingly.
func (f *File) Exposures(shape Shape, v View) (out []Exposure, truncated bool) {
	var reads, records []Exposure
	for _, t := range sortedKeys(v.Entities) {
		fields := f.labeledFields(t, f.Assign[t], v.Entities[t])
		reads = append(reads, f.labelExposures(t, fields)...)
		records = append(records, f.recordExposures(t, fields, func(exclude []string) []Field {
			return withoutRefs(fields, exclude)
		})...)
	}
	for _, r := range sortedKeys(v.Relations) {
		fields := f.labeledFields("~"+r, f.AssignRelations[r], v.Relations[r])
		reads = append(reads, f.labelExposures("~"+r, fields)...)
		records = append(records, f.recordExposures("~"+r, fields, func(exclude []string) []Field {
			return withoutRefs(fields, exclude)
		})...)
	}
	subjects := f.subjectExposures(shape, v)

	budget := MaxFindingsPerView
	take := func(es []Exposure) []Exposure {
		if len(es) > budget {
			es, truncated = es[:budget], true
		}
		budget -= len(es)
		return es
	}
	subjects = take(subjects)
	records = take(records)
	reads = take(reads)
	out = make([]Exposure, 0, len(reads)+len(records)+len(subjects))
	out = append(append(append(out, reads...), records...), subjects...)
	return out, truncated
}

// labelExposures lists each label the fields carry, with the fields that
// carry it.
func (f *File) labelExposures(subject string, fields []Field) []Exposure {
	byLabel := map[string][]string{}
	for _, field := range fields {
		for _, l := range field.Labels {
			byLabel[l] = append(byLabel[l], field.Ref)
		}
	}
	labels := f.inDeclarationOrder(sortedKeys(byLabel))
	out := make([]Exposure, 0, len(labels))
	for _, l := range labels {
		out = append(out, Exposure{Rule: RuleReadsLabel, Subject: subject, Label: l, Fields: byLabel[l]})
	}
	return out
}

func (f *File) recordExposures(subject string, fields []Field, rebuild func(exclude []string) []Field) []Exposure {
	ds := f.Derivations(ScopeRecord, fields)
	out := make([]Exposure, 0, len(ds))
	for _, d := range ds {
		e := Exposure{Rule: RuleDerivedRecord, Subject: subject, Label: d.Label, Fields: d.Fields}
		f.fillBreakers(&e, ScopeRecord, d, rebuild)
		out = append(out, e)
	}
	return out
}

// subjectExposures evaluates subject-scope labels over each subject's
// profile as the view sees it: only readable types, readable relation
// pairs, and visible fields.
func (f *File) subjectExposures(shape Shape, v View) []Exposure {
	visibleShape := Shape{Entities: map[string]TypeShape{}, Relations: map[string]RelationShape{}}
	for t, fields := range v.Entities {
		visibleShape.Entities[t] = TypeShape{Fields: fields}
	}
	for r, ends := range v.Ends {
		visibleShape.Relations[r] = RelationShape{Fields: v.Relations[r], Ends: ends}
	}
	// Subjecthood is a property of the data, not of what a view can read,
	// so it comes from the full schema.
	subjects := map[string]bool{}
	for _, s := range f.Subjects(shape) {
		subjects[s.Type] = true
	}
	links := map[string]RelationShape{}
	for name, r := range visibleShape.Relations {
		if f.isSubjectLink(name, shape.Relations[name], subjects) {
			links[name] = r
		}
	}
	w := &walker{f: f, subjects: subjects, links: links, linkNames: sortedKeys(links)}

	var out []Exposure
	for _, s := range sortedKeys(subjects) {
		if _, readable := visibleShape.Entities[s]; !readable {
			continue
		}
		p := w.profile(s)
		fields := f.ProfileFields(p, visibleShape)
		for _, d := range f.Derivations(ScopeSubject, fields) {
			e := Exposure{Rule: RuleDerivedSubject, Subject: subjectPrefix + s, Label: d.Label, Fields: d.Fields}
			f.fillBreakers(&e, ScopeSubject, d, func(exclude []string) []Field {
				return f.ProfileFields(p, withoutShapeFields(visibleShape, exclude))
			})
			out = append(out, e)
		}
	}
	return out
}

// fillBreakers sets e's Breakers: the contributing fields whose removal
// alone stops d's label from holding. Rules are monotone, so only a
// contributor can break one. When none does, it searches for the smallest
// set that does, up to [MaxRemovals] fields.
func (f *File) fillBreakers(e *Exposure, scope Scope, d Derivation, rebuild func(exclude []string) []Field) {
	breaks := func(exclude []string) bool {
		return !slices.Contains(f.Derive(scope, rebuild(exclude)), d.Label)
	}
	for _, ref := range d.Fields {
		if breaks([]string{ref}) {
			if len(e.Breakers) == MaxBreakers {
				e.BreakersTruncated = true
				break
			}
			e.Breakers = append(e.Breakers, ref)
		}
	}
	if len(e.Breakers) > 0 {
		e.MinRemovals = 1
		return
	}
	if len(d.Fields) > maxRemovalFields {
		return
	}
	for k := 2; k <= MaxRemovals && k <= len(d.Fields); k++ {
		if anySubset(d.Fields, k, breaks) {
			e.MinRemovals = k
			return
		}
	}
}

// anySubset reports whether pred holds for some k-element subset of refs.
func anySubset(refs []string, k int, pred func([]string) bool) bool {
	pick := make([]string, 0, k)
	var walk func(start int) bool
	walk = func(start int) bool {
		if len(pick) == k {
			return pred(pick)
		}
		for i := start; i <= len(refs)-(k-len(pick)); i++ {
			pick = append(pick, refs[i])
			if walk(i + 1) {
				return true
			}
			pick = pick[:len(pick)-1]
		}
		return false
	}
	return walk(0)
}

func withoutRefs(fields []Field, refs []string) []Field {
	out := make([]Field, 0, len(fields))
	for _, field := range fields {
		if !slices.Contains(refs, field.Ref) {
			out = append(out, field)
		}
	}
	return out
}

// withoutShapeFields returns shape without the fields refs name
// ("type.field" or "~relation.field").
func withoutShapeFields(shape Shape, refs []string) Shape {
	out := Shape{Entities: make(map[string]TypeShape, len(shape.Entities)), Relations: shape.Relations}
	for t, ts := range shape.Entities {
		out.Entities[t] = TypeShape{Fields: dropFields(ts.Fields, t, refs)}
	}
	if slices.ContainsFunc(refs, func(r string) bool { return strings.HasPrefix(r, "~") }) {
		out.Relations = make(map[string]RelationShape, len(shape.Relations))
		for r, rs := range shape.Relations {
			out.Relations[r] = RelationShape{Fields: dropFields(rs.Fields, "~"+r, refs), Ends: rs.Ends}
		}
	}
	return out
}

func dropFields(fields []string, prefix string, refs []string) []string {
	out := make([]string, 0, len(fields))
	for _, name := range fields {
		if !slices.Contains(refs, prefix+"."+name) {
			out = append(out, name)
		}
	}
	return out
}
