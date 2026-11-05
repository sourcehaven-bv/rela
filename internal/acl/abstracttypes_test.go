package acl_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

func abstractMeta() fakeMeta {
	return fakeMeta{
		types: map[string]map[string]acl.PropertyInfo{
			"isms.task": {}, "projects.task": {}, "person": {},
		},
		variants: map[string][]string{"task": {"isms.task", "projects.task"}},
	}
}

// TestValidateAgainstMetamodel_ExpandsAbstractTypes pins that a grant on an
// abstract type reaches every variant. Without the expansion it would match
// nothing, because an abstract type stores no entities.
func TestValidateAgainstMetamodel_ExpandsAbstractTypes(t *testing.T) {
	p, err := acl.LoadPolicyBytes([]byte(`
roles:
  editor:
    read: [task, person]
    create: [task]
    fields:
      task: [{field: title}]
      isms.task: [{field: evidence}]
client_baselines:
  apps:
    applies_to: [app]
    redact:
      task: [due]
      isms.task: [secret]
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateAgainstMetamodel(abstractMeta()); err != nil {
		t.Fatal(err)
	}
	role := p.Roles["editor"]
	if want := []string{"isms.task", "projects.task", "person"}; !slices.Equal(role.Read, want) {
		t.Errorf("read = %v, want %v", role.Read, want)
	}
	if want := []string{"isms.task", "projects.task"}; !slices.Equal(role.Create, want) {
		t.Errorf("create = %v, want %v", role.Create, want)
	}
	if _, ok := role.Fields["task"]; ok {
		t.Error("abstract key kept in fields")
	}
	if got := len(role.Fields["isms.task"]); got != 2 {
		t.Errorf("isms.task fields = %v, want own grant plus the abstract one", role.Fields["isms.task"])
	}
	if got := len(role.Fields["projects.task"]); got != 1 {
		t.Errorf("projects.task fields = %v", role.Fields["projects.task"])
	}
	redact := p.ClientBaselines["apps"].Redact
	if !slices.Equal(redact["isms.task"], []string{"secret", "due"}) {
		t.Errorf("variant redact = %v, want its own field and the abstract one", redact["isms.task"])
	}
	if !slices.Equal(redact["projects.task"], []string{"due"}) {
		t.Errorf("inherited redact = %v", redact["projects.task"])
	}

	// A second run changes nothing.
	if err := p.ValidateAgainstMetamodel(abstractMeta()); err != nil {
		t.Fatal(err)
	}
	if got := p.Roles["editor"].Read; len(got) != 3 {
		t.Errorf("second run read = %v", got)
	}
}

func TestValidateAgainstMetamodel_AbstractUserType(t *testing.T) {
	p, err := acl.LoadPolicyBytes([]byte("user_entity_type: task\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = p.ValidateAgainstMetamodel(abstractMeta())
	if err == nil || !strings.Contains(err.Error(), "abstract") {
		t.Fatalf("err = %v, want an abstract-type error", err)
	}
}
