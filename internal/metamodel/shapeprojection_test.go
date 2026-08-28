package metamodel

import "testing"

// shapeFixture builds a small but representative metamodel covering entity
// properties, relation shape, and named enum types.
func shapeFixture() *Metamodel {
	one := 1
	return &Metamodel{
		Types: map[string]CustomType{
			"status": {
				Values:  []string{"todo", "doing", "done"},
				Labels:  map[string]string{"todo": "To do"},
				Default: "todo",
			},
		},
		Entities: map[string]EntityDef{
			"task": {
				Label:           "Task",
				Description:     "a task",
				Color:           "#fff",
				IDPrefix:        "TSK",
				DisplayProperty: "title",
				Properties: map[string]PropertyDef{
					"title":    {Type: "string", Required: true},
					"status":   {Type: "status", Default: "todo"},
					"due":      {Type: "date", Format: "2006-01-02"},
					"tags":     {Type: "string", List: true},
					"priority": {Type: "enum", Values: []string{"low", "high"}},
				},
			},
			"person": {
				Label:      "Person",
				Properties: map[string]PropertyDef{"name": {Type: "string", Required: true}},
			},
		},
		Relations: map[string]RelationDef{
			"assigned-to": {
				Label:       "Assigned to",
				From:        []string{"task"},
				To:          []string{"person"},
				MaxOutgoing: &one,
				Content:     true,
				Properties:  map[string]PropertyDef{"weight": {Type: "integer"}},
			},
		},
	}
}

func TestShapeProjectionHash_Deterministic(t *testing.T) {
	a := shapeFixture().ShapeProjection()
	b := shapeFixture().ShapeProjection()
	if a.Hash() != b.Hash() {
		t.Fatalf("hash not deterministic across equal metamodels: %s vs %s", a.Hash(), b.Hash())
	}
}

func TestShapeProjectionHash_DistinctFromRenderHash(t *testing.T) {
	m := shapeFixture()
	if m.ShapeProjection().Hash() == m.RenderProjection().Hash() {
		t.Fatalf("shape hash and render hash must live in distinct hash spaces")
	}
}

func TestShapeProjectionHash_CosmeticEditsDoNotMoveIt(t *testing.T) {
	base := shapeFixture().ShapeProjection().Hash()

	cosmetic := map[string]func(m *Metamodel){
		"entity label": func(m *Metamodel) {
			d := m.Entities["task"]
			d.Label = "Renamed Task"
			m.Entities["task"] = d
		},
		"entity description": func(m *Metamodel) {
			d := m.Entities["task"]
			d.Description = "different prose"
			m.Entities["task"] = d
		},
		"entity color": func(m *Metamodel) {
			d := m.Entities["task"]
			d.Color = "#000"
			m.Entities["task"] = d
		},
		"entity id prefix": func(m *Metamodel) {
			d := m.Entities["task"]
			d.IDPrefix = "TASK"
			m.Entities["task"] = d
		},
		"entity display property": func(m *Metamodel) {
			d := m.Entities["task"]
			d.DisplayProperty = ""
			m.Entities["task"] = d
		},
		"entity default sort": func(m *Metamodel) {
			d := m.Entities["task"]
			d.DefaultSort = []SortSpec{{Property: "title"}}
			m.Entities["task"] = d
		},
		"entity aliases": func(m *Metamodel) {
			d := m.Entities["task"]
			d.Aliases = []string{"todo-item"}
			m.Entities["task"] = d
		},
		"property description": func(m *Metamodel) {
			p := m.Entities["task"].Properties["title"]
			p.Description = "the title"
			m.Entities["task"].Properties["title"] = p
		},
		"property labels": func(m *Metamodel) {
			p := m.Entities["task"].Properties["priority"]
			p.Labels = map[string]string{"low": "Low"}
			m.Entities["task"].Properties["priority"] = p
		},
		"property unique": func(m *Metamodel) {
			p := m.Entities["task"].Properties["title"]
			p.Unique = true
			m.Entities["task"].Properties["title"] = p
		},
		"custom type labels": func(m *Metamodel) {
			ct := m.Types["status"]
			ct.Labels = map[string]string{"done": "Done!"}
			m.Types["status"] = ct
		},
		"custom type default": func(m *Metamodel) {
			ct := m.Types["status"]
			ct.Default = "doing"
			m.Types["status"] = ct
		},
		"custom type transitions": func(m *Metamodel) {
			ct := m.Types["status"]
			ct.Transitions = []TransitionDef{{From: "todo", To: "doing"}}
			m.Types["status"] = ct
		},
		"relation label": func(m *Metamodel) {
			r := m.Relations["assigned-to"]
			r.Label = "Assignee"
			m.Relations["assigned-to"] = r
		},
		"validations": func(m *Metamodel) {
			m.Validations = []ValidationRule{{}}
		},
		"automations": func(m *Metamodel) {
			m.Automations = []AutomationDef{{}}
		},
	}

	for name, mutate := range cosmetic {
		t.Run(name, func(t *testing.T) {
			m := shapeFixture()
			mutate(m)
			if got := m.ShapeProjection().Hash(); got != base {
				t.Errorf("cosmetic edit %q moved the shape hash", name)
			}
		})
	}
}

func TestShapeProjectionHash_ShapeEditsMoveIt(t *testing.T) {
	base := shapeFixture().ShapeProjection().Hash()

	shape := map[string]func(m *Metamodel){
		"property type": func(m *Metamodel) {
			p := m.Entities["task"].Properties["due"]
			p.Type = "string"
			m.Entities["task"].Properties["due"] = p
		},
		"property required": func(m *Metamodel) {
			p := m.Entities["task"].Properties["status"]
			p.Required = true
			m.Entities["task"].Properties["status"] = p
		},
		"property list": func(m *Metamodel) {
			p := m.Entities["task"].Properties["title"]
			p.List = true
			m.Entities["task"].Properties["title"] = p
		},
		"property format": func(m *Metamodel) {
			p := m.Entities["task"].Properties["due"]
			p.Format = "02-01-2006"
			m.Entities["task"].Properties["due"] = p
		},
		"property inline values": func(m *Metamodel) {
			p := m.Entities["task"].Properties["priority"]
			p.Values = []string{"low", "medium", "high"}
			m.Entities["task"].Properties["priority"] = p
		},
		"property default": func(m *Metamodel) {
			p := m.Entities["task"].Properties["status"]
			p.Default = "doing"
			m.Entities["task"].Properties["status"] = p
		},
		"property added": func(m *Metamodel) {
			m.Entities["task"].Properties["estimate"] = PropertyDef{Type: "integer"}
		},
		"property removed": func(m *Metamodel) {
			delete(m.Entities["task"].Properties, "tags")
		},
		"entity type added": func(m *Metamodel) {
			m.Entities["note"] = EntityDef{Properties: map[string]PropertyDef{"body": {Type: "string"}}}
		},
		"entity type removed": func(m *Metamodel) {
			delete(m.Entities, "person")
		},
		"named type values": func(m *Metamodel) {
			ct := m.Types["status"]
			ct.Values = []string{"todo", "doing", "blocked", "done"}
			m.Types["status"] = ct
		},
		"relation from": func(m *Metamodel) {
			r := m.Relations["assigned-to"]
			r.From = []string{"task", "note"}
			m.Relations["assigned-to"] = r
		},
		"relation cardinality": func(m *Metamodel) {
			r := m.Relations["assigned-to"]
			r.MaxOutgoing = nil
			m.Relations["assigned-to"] = r
		},
		"relation symmetric": func(m *Metamodel) {
			r := m.Relations["assigned-to"]
			r.Symmetric = true
			m.Relations["assigned-to"] = r
		},
		"relation content": func(m *Metamodel) {
			r := m.Relations["assigned-to"]
			r.Content = false
			m.Relations["assigned-to"] = r
		},
		"relation property": func(m *Metamodel) {
			r := m.Relations["assigned-to"]
			r.Properties["weight"] = PropertyDef{Type: "string"}
			m.Relations["assigned-to"] = r
		},
		"relation type added": func(m *Metamodel) {
			m.Relations["blocks"] = RelationDef{From: []string{"task"}, To: []string{"task"}}
		},
	}

	for name, mutate := range shape {
		t.Run(name, func(t *testing.T) {
			m := shapeFixture()
			mutate(m)
			if got := m.ShapeProjection().Hash(); got == base {
				t.Errorf("shape edit %q did not move the shape hash", name)
			}
		})
	}
}

func TestShapeProjection_JSONRoundTrip(t *testing.T) {
	orig := shapeFixture().ShapeProjection()
	data, err := orig.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	parsed, err := ShapeProjectionFromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	if parsed.Hash() != orig.Hash() {
		t.Fatalf("round-tripped projection hashes differently: %s vs %s", parsed.Hash(), orig.Hash())
	}
}

func TestShapeProjection_HashDistinguishesAbsentFromZeroCardinality(t *testing.T) {
	zero := 0
	a := shapeFixture()
	b := shapeFixture()
	r := b.Relations["assigned-to"]
	r.MinOutgoing = &zero
	b.Relations["assigned-to"] = r
	if a.ShapeProjection().Hash() == b.ShapeProjection().Hash() {
		t.Fatalf("absent and zero cardinality bounds must hash differently")
	}
}
