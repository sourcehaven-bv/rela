package classification

import (
	"slices"
	"testing"
)

// fullView reads everything in workedShape.
func fullView() View {
	v := View{Entities: map[string][]string{}, Relations: map[string][]string{}, Ends: map[string][]Ends{}}
	s := workedShape()
	for t, ts := range s.Entities {
		v.Entities[t] = ts.Fields
	}
	for r, rs := range s.Relations {
		v.Relations[r] = rs.Fields
		v.Ends[r] = rs.Ends
	}
	return v
}

func findExposure(es []Exposure, rule, subject, label string) *Exposure {
	for i := range es {
		if es[i].Rule == rule && es[i].Subject == subject && es[i].Label == label {
			return &es[i]
		}
	}
	return nil
}

func TestExposures_RecordCombination(t *testing.T) {
	f := mustParse(t, workedFile)
	v := View{Entities: map[string][]string{"employment": {"birth_date", "postcode", "gender"}}}
	es, truncated := f.Exposures(workedShape(), v)
	if truncated {
		t.Fatal("unexpected truncation")
	}
	e := findExposure(es, RuleDerivedRecord, "employment", "identified-person")
	if e == nil {
		t.Fatalf("no C2 finding in %+v", es)
	}
	want := []string{"employment.birth_date", "employment.gender", "employment.postcode"}
	if !slices.Equal(e.Breakers, want) {
		t.Errorf("breakers = %v, want %v (count of 3 needs all three)", e.Breakers, want)
	}
	if c1 := findExposure(es, RuleReadsLabel, "employment", "postcode"); c1 == nil {
		t.Errorf("no C1 finding for postcode in %+v", es)
	}
	if findExposure(es, RuleReadsLabel, "employment", "salary") != nil {
		t.Error("salary is not visible to this view")
	}
}

func TestExposures_ClientRemovesField(t *testing.T) {
	f := mustParse(t, workedFile)
	v := View{Entities: map[string][]string{"employment": {"birth_date", "gender"}}}
	es, _ := f.Exposures(workedShape(), v)
	if findExposure(es, RuleDerivedRecord, "employment", "identified-person") != nil {
		t.Errorf("without postcode, identified-person must not hold: %+v", es)
	}
}

func TestExposures_NoSingleBreaker(t *testing.T) {
	// any_of with two independent routes: removing any one field leaves the
	// other route standing.
	f := mustParse(t, `
labels:
  a: {}
  b: {}
  d:
    when: { any_of: [a, b] }
assign:
  t: { x: [a], y: [b] }
`)
	shape := Shape{Entities: map[string]TypeShape{"t": {Fields: []string{"x", "y"}}}}
	es, _ := f.Exposures(shape, View{Entities: map[string][]string{"t": {"x", "y"}}})
	e := findExposure(es, RuleDerivedRecord, "t", "d")
	if e == nil || len(e.Breakers) != 0 || e.MinRemovals != 2 {
		t.Errorf("exposure = %+v; no single removal breaks d, two do", e)
	}
}

func TestExposures_Subject(t *testing.T) {
	f := mustParse(t, workedFile)
	es, _ := f.Exposures(workedShape(), fullView())
	e := findExposure(es, RuleDerivedSubject, "subject person", "identified-health")
	if e == nil {
		t.Fatalf("no C3 finding in %+v", es)
	}
	if !slices.Contains(e.Breakers, "sick-leave.diagnosis") {
		t.Errorf("breakers = %v; diagnosis is the only health field", e.Breakers)
	}

	t.Run("without the link", func(t *testing.T) {
		v := fullView()
		delete(v.Relations, "of")
		delete(v.Ends, "of")
		es, _ := f.Exposures(workedShape(), v)
		if findExposure(es, RuleDerivedSubject, "subject person", "identified-health") != nil {
			t.Error("without a readable of edge, health is not linked to the person")
		}
	})
	t.Run("without the diagnosis", func(t *testing.T) {
		v := fullView()
		v.Entities["sick-leave"] = []string{"body"}
		es, _ := f.Exposures(workedShape(), v)
		if findExposure(es, RuleDerivedSubject, "subject person", "identified-health") != nil {
			t.Error("a hidden diagnosis cannot combine")
		}
	})
}

func TestExposures_Relations(t *testing.T) {
	f := mustParse(t, workedFile+"  reports:\n    note: [health]\n")
	shape := workedShape()
	shape.Relations["reports"] = RelationShape{Fields: []string{"note"}, Ends: []Ends{{From: "person", To: "company"}}}
	v := View{Relations: map[string][]string{"reports": {"note"}}}
	es, _ := f.Exposures(shape, v)
	if findExposure(es, RuleReadsLabel, "~reports", "health") == nil {
		t.Errorf("no C1 finding on the relation field: %+v", es)
	}
}

func TestExposures_Truncated(t *testing.T) {
	f := mustParse(t, "labels: {a: {}}\nassign: {}\n")
	shape := Shape{Entities: map[string]TypeShape{}}
	v := View{Entities: map[string][]string{}}
	for i := range MaxFindingsPerView + 1 {
		name := "t" + itoa3(i)
		shape.Entities[name] = TypeShape{Fields: []string{"x"}}
		v.Entities[name] = []string{"x"}
		f.Assign[name] = TypeAssignments{Fields: map[string]Assignment{"x": {State: Labeled, Labels: []string{"a"}}}}
	}
	es, truncated := f.Exposures(shape, v)
	if !truncated || len(es) != MaxFindingsPerView {
		t.Errorf("truncated=%v len=%d", truncated, len(es))
	}

	// Combinations are kept before plain reads.
	f.Labels["d"] = &Label{When: &Rule{Scope: ScopeRecord, cond: cond{kind: condAnyOf, children: []cond{
		{kind: condSelector, sel: selector{label: "a"}},
	}}}}
	f.LabelOrder = append(f.LabelOrder, "d")
	es, _ = f.Exposures(shape, v)
	for _, e := range es {
		if e.Rule != RuleDerivedRecord {
			t.Fatalf("kept a %s finding while C2 findings were cut", e.Rule)
		}
	}
}

func TestExposures_MinRemovalsBeyondSearch(t *testing.T) {
	f := mustParse(t, `
labels:
  a: {}
  d:
    when: { any_of: [a] }
assign:
  t: { w: [a], x: [a], y: [a], z: [a] }
`)
	shape := Shape{Entities: map[string]TypeShape{"t": {Fields: []string{"w", "x", "y", "z"}}}}
	es, _ := f.Exposures(shape, View{Entities: map[string][]string{"t": {"w", "x", "y", "z"}}})
	e := findExposure(es, RuleDerivedRecord, "t", "d")
	if e == nil || len(e.Breakers) != 0 || e.MinRemovals != 0 {
		t.Errorf("exposure = %+v; four removals exceed the search", e)
	}
}
