package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func newDefaultViewMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"status": {Type: "string"},
				},
				PropertyOrder: []string{"title", "status"},
			},
			"feature": {
				Label: "Feature",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
				},
				PropertyOrder: []string{"title"},
			},
			"person": {
				Label:         "Person",
				Properties:    map[string]metamodel.PropertyDef{},
				PropertyOrder: []string{},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"implements": {
				Label: "implements",
				From:  []string{"ticket"},
				To:    []string{"feature"},
			},
			"blocks": {
				Label: "blocks",
				From:  []string{"ticket"},
				To:    []string{"ticket"},
			},
			"knows": {
				Label:     "knows",
				From:      []string{"person"},
				To:        []string{"person"},
				Symmetric: true,
			},
			"affects": {
				Label: "affects",
				From:  []string{"ticket"},
				To:    []string{"feature"},
				Inverse: &metamodel.InverseDef{
					ID:    "affectedBy",
					Label: "affected by",
				},
			},
		},
	}
}

func TestBuildDefaultViewConfig_UnknownType(t *testing.T) {
	meta := newDefaultViewMetamodel()
	_, ok := buildDefaultViewConfig(meta, "nonexistent")
	if ok {
		t.Fatal("expected ok=false for unknown entity type")
	}
}

func TestBuildDefaultViewConfig_PropertiesAndContent(t *testing.T) {
	meta := newDefaultViewMetamodel()
	view, ok := buildDefaultViewConfig(meta, "feature")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if view.Entry.Type != "feature" {
		t.Errorf("entry.type: want feature, got %q", view.Entry.Type)
	}
	if view.Title != "Feature" {
		t.Errorf("title: want Feature, got %q", view.Title)
	}

	// Properties and content always lead, in that order.
	if view.Sections[0].Display != "properties" {
		t.Errorf("section[0].display: want properties, got %q", view.Sections[0].Display)
	}
	if view.Sections[1].Display != "content" {
		t.Errorf("section[1].display: want content, got %q", view.Sections[1].Display)
	}
}

func TestBuildDefaultViewConfig_NoPropertiesOmitsSection(t *testing.T) {
	meta := newDefaultViewMetamodel()
	view, ok := buildDefaultViewConfig(meta, "person")
	if !ok {
		t.Fatal("expected ok=true")
	}
	// person has no properties, no non-symmetric relations — only the
	// content section (and the symmetric "knows" outgoing) should appear.
	for _, sec := range view.Sections {
		if sec.Display == "properties" {
			t.Errorf("expected no properties section for person, got: %+v", sec)
		}
	}
}

func TestBuildDefaultViewConfig_PropertyOrderPreserved(t *testing.T) {
	meta := newDefaultViewMetamodel()
	view, _ := buildDefaultViewConfig(meta, "ticket")
	var propsSec *ViewSection
	for i := range view.Sections {
		if view.Sections[i].Display == "properties" {
			propsSec = &view.Sections[i]
			break
		}
	}
	if propsSec == nil {
		t.Fatal("expected a properties section")
	}
	if got, want := len(propsSec.Fields), 2; got != want {
		t.Fatalf("fields: want %d, got %d", want, got)
	}
	if propsSec.Fields[0].Property != "title" || propsSec.Fields[1].Property != "status" {
		t.Errorf("field order: want [title, status], got [%s, %s]",
			propsSec.Fields[0].Property, propsSec.Fields[1].Property)
	}
}

func TestBuildDefaultViewConfig_OutgoingRelationSection(t *testing.T) {
	meta := newDefaultViewMetamodel()
	view, _ := buildDefaultViewConfig(meta, "ticket")

	// ticket --implements--> feature: outgoing section expected.
	var found bool
	for _, sec := range view.Sections {
		if sec.Source == "out_implements" {
			found = true
			if sec.Display != "cards" {
				t.Errorf("display: want cards, got %q", sec.Display)
			}
			if sec.Heading != "implements" {
				t.Errorf("heading: want %q, got %q", "implements", sec.Heading)
			}
		}
	}
	if !found {
		t.Errorf("expected section with source=out_implements; got sections: %+v", view.Sections)
	}

	// And a matching traverse rule.
	var traverseFound bool
	for _, tr := range view.Traverse {
		if tr.CollectAs == "out_implements" {
			traverseFound = true
			if tr.Follow != "implements" || tr.From != "entry" {
				t.Errorf("traverse mismatch: %+v", tr)
			}
		}
	}
	if !traverseFound {
		t.Error("expected matching traverse rule for out_implements")
	}
}

func TestBuildDefaultViewConfig_IncomingRelationSectionUsesInverseLabel(t *testing.T) {
	meta := newDefaultViewMetamodel()
	view, _ := buildDefaultViewConfig(meta, "feature")

	// ticket --affects--> feature, inverse "affected by".
	for _, sec := range view.Sections {
		if sec.Source == "in_affects" {
			if sec.Heading != "affected by" {
				t.Errorf("heading should use inverse label: want %q, got %q", "affected by", sec.Heading)
			}
			return
		}
	}
	t.Errorf("expected section with source=in_affects; got: %+v", view.Sections)
}

func TestBuildDefaultViewConfig_SelfReferentialRelationEmitsBothDirections(t *testing.T) {
	meta := newDefaultViewMetamodel()
	view, _ := buildDefaultViewConfig(meta, "ticket")

	var hasOut, hasIn bool
	for _, sec := range view.Sections {
		if sec.Source == "out_blocks" {
			hasOut = true
		}
		if sec.Source == "in_blocks" {
			hasIn = true
		}
	}
	if !hasOut {
		t.Error("expected outgoing blocks section")
	}
	if !hasIn {
		t.Error("expected incoming blocks section (self-referential)")
	}
}

func TestBuildDefaultViewConfig_SymmetricRelationOnlyOutgoing(t *testing.T) {
	meta := newDefaultViewMetamodel()
	view, _ := buildDefaultViewConfig(meta, "person")

	var outCount, inCount int
	for _, sec := range view.Sections {
		if sec.Source == "out_knows" {
			outCount++
		}
		if sec.Source == "in_knows" {
			inCount++
		}
	}
	if outCount != 1 {
		t.Errorf("outgoing knows: want 1, got %d", outCount)
	}
	if inCount != 0 {
		t.Errorf("symmetric relation should not emit incoming section: got %d", inCount)
	}
}

func TestBuildDefaultViewConfig_SectionOrderingDeterministic(t *testing.T) {
	meta := newDefaultViewMetamodel()
	// Run twice; sections must be identical (map iteration order
	// otherwise would shuffle relation sections).
	first, _ := buildDefaultViewConfig(meta, "ticket")
	for range 5 {
		again, _ := buildDefaultViewConfig(meta, "ticket")
		if len(again.Sections) != len(first.Sections) {
			t.Fatal("section count differs across runs")
		}
		for i := range again.Sections {
			if again.Sections[i].Source != first.Sections[i].Source {
				t.Errorf("section[%d].source differs: %q vs %q",
					i, again.Sections[i].Source, first.Sections[i].Source)
			}
		}
	}
}
