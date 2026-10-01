package classification

// Field states as the report names them.
const (
	reportLabeled = "labeled"
	reportMissing = "missing"
)

// Report is what the classification says about the schema: labels per
// field, derived labels per record and per subject, and which types and
// relations make up each subject's data. Every list is sorted, so the JSON
// form is stable.
type Report struct {
	Types     []TypeReport     `json:"types"`
	Relations []RelationReport `json:"relations"`
	Subjects  []SubjectReport  `json:"subjects"`
}

// FieldReport is one field's entry.
type FieldReport struct {
	Field string `json:"field"`
	// State is labeled, none, needs-review, or missing (no entry).
	State  string   `json:"state"`
	Labels []string `json:"labels,omitempty"`
}

// TypeReport describes one entity type.
type TypeReport struct {
	Type    string        `json:"type"`
	Fields  []FieldReport `json:"fields"`
	Derived []Derivation  `json:"derived,omitempty"`
	// Subject is the provenance when the type is a subject, else empty.
	Subject string `json:"subject,omitempty"`
	// IDMayIdentify is set on a subject type whose ids are not opaque: a
	// manual id such as "jane-doe" is personal data no field entry covers.
	IDMayIdentify bool `json:"id_may_identify,omitempty"`
}

// RelationReport describes one relation type.
type RelationReport struct {
	Relation    string        `json:"relation"`
	Fields      []FieldReport `json:"fields"`
	Derived     []Derivation  `json:"derived,omitempty"`
	SubjectLink bool          `json:"subject_link"`
}

// SubjectReport is a subject's profile and the subject-scope labels it
// carries.
type SubjectReport struct {
	Profile
	Derived []Derivation `json:"derived,omitempty"`
}

// BuildReport describes f against shape. It works on a file with lint
// issues too: a missing entry is reported as missing and contributes no
// labels.
func BuildReport(f *File, shape Shape) Report {
	subjects := map[string]string{}
	for _, s := range f.Subjects(shape) {
		subjects[s.Type] = s.Reason
	}
	rep := Report{Types: []TypeReport{}, Relations: []RelationReport{}, Subjects: []SubjectReport{}}
	for _, name := range sortedKeys(shape.Entities) {
		t := shape.Entities[name]
		reason, isSubject := subjects[name]
		rep.Types = append(rep.Types, TypeReport{
			Type:          name,
			Fields:        fieldReports(f.Assign[name], t.Fields),
			Derived:       f.Derivations(ScopeRecord, f.ownFields(name, t)),
			Subject:       reason,
			IDMayIdentify: isSubject && !t.OpaqueID,
		})
	}
	links := map[string]bool{}
	for name, r := range shape.Relations {
		links[name] = f.isSubjectLink(name, r, boolSet(subjects))
	}
	for _, name := range sortedKeys(shape.Relations) {
		r := shape.Relations[name]
		rep.Relations = append(rep.Relations, RelationReport{
			Relation:    name,
			Fields:      fieldReports(f.AssignRelations[name], r.Fields),
			Derived:     f.Derivations(ScopeRecord, f.relationFields(name, r)),
			SubjectLink: links[name],
		})
	}
	for _, p := range f.Profiles(shape) {
		if p.Reached == nil {
			p.Reached = []Reach{}
		}
		if p.Relations == nil {
			p.Relations = []string{}
		}
		rep.Subjects = append(rep.Subjects, SubjectReport{
			Profile: p,
			Derived: f.Derivations(ScopeSubject, f.ProfileFields(p, shape)),
		})
	}
	return rep
}

func fieldReports(ta TypeAssignments, fields []string) []FieldReport {
	out := make([]FieldReport, 0, len(fields))
	for _, name := range fields {
		a, ok := ta.Fields[name]
		fr := FieldReport{Field: name}
		switch {
		case !ok:
			fr.State = reportMissing
		case a.State == Labeled:
			fr.State, fr.Labels = reportLabeled, a.Labels
		case a.State == None:
			fr.State = StateNone
		case a.State == NeedsReview:
			fr.State = StateNeedsReview
		}
		out = append(out, fr)
	}
	return out
}

func boolSet[V any](m map[string]V) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}
