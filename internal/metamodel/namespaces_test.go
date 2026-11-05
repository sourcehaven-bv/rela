package metamodel

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const nsRoot = `
version: "1.0"
iri: "https://example.org/ontology#"
includes: [isms.yaml, projects.yaml]
types:
  task_status:
    values: [open, done]
    default: open
entities:
  person:
    label: Person
    id_prefix: "P-"
    properties:
      name: { type: string, required: true }
  task:
    abstract:
      conversions: true
    label: Task
    plural: tasks
    faces:
      draft: {}
      published: {}
    properties:
      title: { type: string, required: true }
      status: { type: task_status }
      due: { type: date }
relations:
  assigned-to:
    label: assigned to
    from: [task]
    to: [person]
validations:
  - name: task-has-title
    entity_type: task
    then: ["title!="]
automations:
  - name: stamp-done
    on: { entity: task, property: status, becomes: done }
    do:
      - set: due
        value: "{{today}}"
comments:
  enabled: true
  on: [task]
`

const nsISMS = `
namespace: isms
types:
  evidence_kind:
    values: [doc, log]
entities:
  task:
    extends: task
    label: ISMS task
    id_prefix: "ISMS-"
    properties:
      evidence: { type: evidence_kind }
      title: { description: "What must be done" }
  control:
    label: Control
    id_prefix: "CTL-"
    properties:
      title: { type: string, required: true }
relations:
  addresses:
    label: addresses
    from: [task]
    to: [control]
    min_outgoing: 1
    inverse: { id: addressed-by }
validations:
  - name: evidence-set
    entity_type: task
    then: ["evidence!="]
automations:
  - name: link-control
    on: { entity: task, created: true }
    do:
      - create_entity: { type: control, relation: addresses }
`

const nsProjects = `
namespace: projects
entities:
  task:
    extends: task
    label: Project task
    id_prefix: "PRJ-"
    properties:
      milestone: { type: string }
`

func loadNS(t *testing.T, files map[string]string) (*Metamodel, error) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		createFile(t, filepath.Join(dir, name), content)
	}
	m, _, err := Load(filepath.Join(dir, "schema.yaml"), testMetaFS)
	return m, err
}

func mustLoadNS(t *testing.T) *Metamodel {
	t.Helper()
	m, err := loadNS(t, map[string]string{"schema.yaml": nsRoot, "isms.yaml": nsISMS, "projects.yaml": nsProjects})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return m
}

func TestNamespaces_QualifyAndResolveLocally(t *testing.T) {
	m := mustLoadNS(t)

	wantTypes := []string{"isms.control", "isms.task", "person", "projects.task"}
	got := m.EntityTypes()
	slices.Sort(got)
	if !slices.Equal(got, wantTypes) {
		t.Fatalf("entity types = %v, want %v (the abstract task is removed)", got, wantTypes)
	}
	if _, ok := m.Types["isms.evidence_kind"]; !ok {
		t.Errorf("custom type not qualified: %v", sortedKeys(m.Types))
	}
	if got := m.Entities["isms.task"].Properties["evidence"].Type; got != "isms.evidence_kind" {
		t.Errorf("local custom type reference = %q, want isms.evidence_kind", got)
	}
	if got := m.Entities["isms.task"].Properties["status"].Type; got != "task_status" {
		t.Errorf("inherited root type reference = %q, want task_status", got)
	}

	addresses, ok := m.Relations["isms.addresses"]
	if !ok {
		t.Fatalf("relation not qualified: %v", sortedKeys(m.Relations))
	}
	if !slices.Equal(addresses.From, []string{"isms.task"}) || !slices.Equal(addresses.To, []string{"isms.control"}) {
		t.Errorf("isms.addresses endpoints = %v -> %v", addresses.From, addresses.To)
	}
	if addresses.Inverse.ID != "isms.addressed-by" {
		t.Errorf("inverse = %q, want isms.addressed-by", addresses.Inverse.ID)
	}
	if owner, ok := m.InverseOwner("isms.addressed-by"); !ok || owner != "isms.addresses" {
		t.Errorf("InverseOwner = %q, %v", owner, ok)
	}
}

func TestNamespaces_BareAbstractNameExpands(t *testing.T) {
	m := mustLoadNS(t)
	variants := []string{"isms.task", "projects.task"}

	if got := m.Relations["assigned-to"].From; !slices.Equal(got, variants) {
		t.Errorf("assigned-to.from = %v, want %v", got, variants)
	}
	if got := m.Comments.On; !slices.Equal(got, variants) {
		t.Errorf("comments.on = %v, want %v", got, variants)
	}
	for _, rule := range m.Validations {
		switch rule.Name {
		case "task-has-title":
			if got := rule.TargetTypes(); !slices.Equal(got, variants) {
				t.Errorf("task-has-title targets %v, want %v", got, variants)
			}
			if rule.AppliesToType("person") {
				t.Error("task-has-title applies to person")
			}
		case "isms.evidence-set":
			if got := rule.TargetTypes(); !slices.Equal(got, []string{"isms.task"}) {
				t.Errorf("isms.evidence-set targets %v", got)
			}
		default:
			t.Errorf("unexpected validation %q", rule.Name)
		}
	}
	for _, a := range m.Automations {
		switch a.Name {
		case "stamp-done":
			if !slices.Equal([]string(a.On.Entity), variants) {
				t.Errorf("stamp-done on %v", a.On.Entity)
			}
		case "isms.link-control":
			if !slices.Equal([]string(a.On.Entity), []string{"isms.task"}) {
				t.Errorf("isms.link-control on %v", a.On.Entity)
			}
			c := a.Do[0].CreateEntity
			if c.Type != "isms.control" || c.Relation != "isms.addresses" {
				t.Errorf("create_entity = %+v", *c)
			}
		default:
			t.Errorf("unexpected automation %q (included automations must merge)", a.Name)
		}
	}

	if got, ok := VariantsOf(m, "task"); !ok || !slices.Equal(got, variants) {
		t.Errorf("VariantsOf(task) = %v, %v", got, ok)
	}
	var abs *AbstractTypeError
	if err := CheckConcreteType(m, "task"); !errors.As(err, &abs) || !strings.Contains(err.Error(), "isms.task") {
		t.Errorf("CheckConcreteType(task) = %v", err)
	}
	if err := CheckConcreteType(m, "isms.task"); err != nil {
		t.Errorf("CheckConcreteType(isms.task) = %v", err)
	}
}

func TestNamespaces_VariantInheritsShape(t *testing.T) {
	m := mustLoadNS(t)
	isms := m.Entities["isms.task"]

	if isms.Extends != "task" || isms.Abstract.Set {
		t.Errorf("extends = %q, abstract = %v", isms.Extends, isms.Abstract)
	}
	if isms.Label != "ISMS task" || isms.IDPrefix != "ISMS-" {
		t.Errorf("own settings lost: label %q prefix %q", isms.Label, isms.IDPrefix)
	}
	if isms.GetPlural("isms.task") != "isms.tasks" {
		t.Errorf("plural = %q, want isms.tasks", isms.GetPlural("isms.task"))
	}
	title := isms.Properties["title"]
	if title.Type != PropertyTypeString || !title.Required || title.Description != "What must be done" {
		t.Errorf("overridden title lost inherited settings: %+v", title)
	}
	wantOrder := []string{"title", "status", "due", "evidence"}
	if got := isms.GetPropertyOrder(); !slices.Equal(got, wantOrder) {
		t.Errorf("property order = %v, want %v", got, wantOrder)
	}
	if got := FaceOrderOf(m, "isms.task"); !slices.Equal(got, []string{"draft", "published"}) {
		t.Errorf("faces = %v", got)
	}
	if _, ok := m.Entities["projects.task"].Properties["evidence"]; ok {
		t.Error("a variant's own property leaked into its sibling")
	}
}

func TestNamespaces_GeneratedVariantConversions(t *testing.T) {
	m := mustLoadNS(t)
	d, ok := m.Conversions["isms.task-to-projects.task"]
	if !ok {
		t.Fatalf("conversions = %v", sortedKeys(m.Conversions))
	}
	if _, ok := m.Conversions["projects.task-to-isms.task"]; !ok {
		t.Error("reverse conversion missing")
	}
	for _, f := range []string{"title", "status", "due"} {
		if d.Fields[f] != "{{source."+f+"}}" {
			t.Errorf("fields.%s = %q", f, d.Fields[f])
		}
	}
	if _, ok := d.Fields["evidence"]; ok {
		t.Error("evidence mapped to a type that lacks it")
	}
	if d.Relations["assigned-to"] != "assigned-to" {
		t.Errorf("assigned-to = %q, want kept", d.Relations["assigned-to"])
	}
	if d.Relations["isms.addresses"] != ConversionDrop {
		t.Errorf("isms.addresses = %q, want drop", d.Relations["isms.addresses"])
	}
	if !slices.Equal(d.Faces["draft"], []string{"draft"}) {
		t.Errorf("faces = %v", d.Faces)
	}
}

func TestNamespaces_HandWrittenConversionWins(t *testing.T) {
	root := nsRoot + `
conversions:
  promote:
    from: isms.task
    to: projects.task
    fields: { title: "{{source.title}}" }
    relations: { assigned-to: assigned-to, isms.addresses: drop }
    faces: { draft: [draft], published: [published] }
`
	m, err := loadNS(t, map[string]string{"schema.yaml": root, "isms.yaml": nsISMS, "projects.yaml": nsProjects})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Conversions["isms.task-to-projects.task"]; ok {
		t.Error("generated conversion shadows the hand-written one")
	}
	if _, ok := m.Conversions["projects.task-to-isms.task"]; !ok {
		t.Error("the other direction should still be generated")
	}
}

func TestNamespaces_LoadErrors(t *testing.T) {
	base := map[string]string{"isms.yaml": nsISMS, "projects.yaml": nsProjects}
	tests := []struct {
		name    string
		root    string
		extra   map[string]string
		wantErr string
	}{
		{
			name:    "dot in a hand-written name",
			root:    strings.Replace(nsRoot, "  person:", "  a.person:", 1),
			wantErr: `entity "a.person": names must not contain "."`,
		},
		{
			name: "extends a concrete type",
			root: nsRoot,
			extra: map[string]string{"projects.yaml": strings.Replace(nsProjects,
				"extends: task", "extends: person", 1)},
			wantErr: `entity "projects.task" (projects.yaml:4) extends "person", which is not abstract`,
		},
		{
			name:    "abstract without variants",
			root:    strings.Replace(nsRoot, "includes: [isms.yaml, projects.yaml]", "includes: []", 1),
			wantErr: `entity "task" (schema.yaml:`,
		},
		{
			name:    "abstract name where one type is needed",
			root:    nsRoot + "conversions:\n  bad:\n    from: task\n    to: person\n    fields: { name: x }\n",
			wantErr: `conversion "bad": from: "task" is abstract and this setting takes one concrete type`,
		},
		{
			name:    "bad namespace name",
			root:    nsRoot,
			extra:   map[string]string{"projects.yaml": strings.Replace(nsProjects, "namespace: projects", "namespace: Pro.jects", 1)},
			wantErr: `projects.yaml: namespace "Pro.jects" must match`,
		},
		{
			name:    "alias equal to an abstract name",
			root:    strings.Replace(nsRoot, "    label: Person\n", "    label: Person\n    aliases: [task]\n", 1),
			wantErr: `alias or plural "task" is the name of an abstract type`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"schema.yaml": tc.root}
			maps.Copy(files, base)
			maps.Copy(files, tc.extra)
			_, err := loadNS(t, files)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestNamespaces_LegacyRootNamespaceIsIRI(t *testing.T) {
	m, err := Parse([]byte(`
version: "1.0"
namespace: "https://example.org/x#"
entities:
  thing:
    label: Thing
    id_prefix: "T-"
    properties:
      title: { type: string }
`))
	if err != nil {
		t.Fatal(err)
	}
	if m.IRI != "https://example.org/x#" || m.Namespace != "" {
		t.Errorf("IRI = %q, Namespace = %q", m.IRI, m.Namespace)
	}
}

func TestNamespaces_QualifiedReferenceAcrossNamespaces(t *testing.T) {
	projects := nsProjects + `
relations:
  follows-up:
    label: follows up
    from: [task]
    to: [isms.task]
`
	m, err := loadNS(t, map[string]string{"schema.yaml": nsRoot, "isms.yaml": nsISMS, "projects.yaml": projects})
	if err != nil {
		t.Fatal(err)
	}
	rel := m.Relations["projects.follows-up"]
	if !slices.Equal(rel.From, []string{"projects.task"}) || !slices.Equal(rel.To, []string{"isms.task"}) {
		t.Errorf("projects.follows-up = %v -> %v", rel.From, rel.To)
	}
}

func TestNamespaces_QualifiesConstraintAndNotifyTargets(t *testing.T) {
	isms := strings.Replace(nsISMS, "validations:\n", `validations:
  - name: task-has-control
    entity_type: task
    relations:
      addresses: { target_type: control, min: 1 }
`, 1)
	m, err := loadNS(t, map[string]string{"schema.yaml": nsRoot, "isms.yaml": isms, "projects.yaml": nsProjects})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, rule := range m.Validations {
		if rule.Name != "isms.task-has-control" {
			continue
		}
		c, ok := rule.Relations["isms.addresses"]
		if !ok || c.TargetType != "isms.control" {
			t.Errorf("constraint = %+v (ok=%v), want target isms.control", c, ok)
		}
		return
	}
	t.Fatal("isms.task-has-control not loaded")
}

func TestQualifyAction_Notify(t *testing.T) {
	q := qualifier{
		ns: "isms", errs: new([]string),
		localEntities:  map[string]bool{"control": true},
		localRelations: map[string]bool{"addresses": true},
	}
	act := AutomationAction{Notify: &NotifyAction{To: NotifyTarget{
		Via: StringOrSlice{"addresses", "owned-by"}, Type: "control", Property: "owner",
	}}}
	qualifyAction(&act, q.entityRef, q.relationRef)
	if got := []string(act.Notify.To.Via); !slices.Equal(got, []string{"isms.addresses", "owned-by"}) {
		t.Errorf("via = %v", got)
	}
	if act.Notify.To.Type != "isms.control" {
		t.Errorf("type = %q", act.Notify.To.Type)
	}
}

func TestNamespaces_RefusesLocalNameInExpression(t *testing.T) {
	isms := strings.Replace(nsISMS, "validations:\n", `validations:
  - name: needs-control
    entity_type: task
    when_condition: "#related('addresses') == 0"
    then: ["title!="]
`, 1)
	_, err := loadNS(t, map[string]string{"schema.yaml": nsRoot, "isms.yaml": isms, "projects.yaml": nsProjects})
	if err == nil || !strings.Contains(err.Error(), `write "isms.addresses"`) {
		t.Fatalf("err = %v, want a request for the qualified name", err)
	}
}

// TestNamespaces_RootNamespaceBecomesIRI pins that any legacy root
// `namespace:` value is read as the IRI, IRI-shaped or not.
func TestNamespaces_RootNamespaceBecomesIRI(t *testing.T) {
	root := strings.Replace(nsRoot, `iri: "https://example.org/ontology#"`, "namespace: myproject", 1)
	m, err := loadNS(t, map[string]string{"schema.yaml": root, "isms.yaml": nsISMS, "projects.yaml": nsProjects})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.IRI != "myproject" || m.Namespace != "" {
		t.Errorf("IRI = %q, Namespace = %q; want IRI myproject and no namespace", m.IRI, m.Namespace)
	}
}

// TestNamespaces_VariantChangesPropertyType pins that a variant which
// redeclares a property with another type does not keep the parent's
// enum values, and that variants do not share the parent's slices.
func TestNamespaces_VariantChangesPropertyType(t *testing.T) {
	isms := strings.Replace(nsISMS, `      title: { description: "What must be done" }`,
		"      title: { description: \"What must be done\" }\n      status: { type: string }", 1)
	m, err := loadNS(t, map[string]string{"schema.yaml": nsRoot, "isms.yaml": isms, "projects.yaml": nsProjects})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := m.Entities["isms.task"].Properties["status"]
	if got.Type != "string" || len(got.Values) != 0 || got.Default != "" {
		t.Errorf("isms.task status = %+v, want a plain string", got)
	}
	ismsDef, projDef := m.Entities["isms.task"], m.Entities["projects.task"]
	if len(ismsDef.Faces) > 0 && len(projDef.Faces) > 0 {
		for k := range ismsDef.Faces {
			delete(ismsDef.Faces, k)
		}
		if len(projDef.Faces) == 0 {
			t.Error("variants share the parent's faces map")
		}
	}
}
