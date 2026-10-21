package appbuild_test

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// copyVisibilityMeta and copyVisibilityPolicy hide person.salary from bob.
// They are local because the field-redaction fixtures are excluded from the
// database and memory builds, and this test runs in every build.
const copyVisibilityMeta = `version: "1.0"
entities:
  person:
    label: Person
    id_prefix: "PERS-"
    properties:
      name: {type: string}
      salary: {type: string}
relations: {}
`

const copyVisibilityPolicy = `roles:
  viewer:
    read: ["*"]
    visible:
      person:
        - field: name
assignments:
  bob: viewer
`

// TestCopyVisibility_NamesHiddenFieldsWithoutValues pins what the task-item
// converter relies on in the wired copy reader: a hidden field is named in
// Redacted whether or not the row stores a value for it. The converter
// refuses on that list, so a reader that named only stored values would tell
// the caller which hidden fields are empty, and one that stripped without
// naming would let a hidden value into the new entity. The entitymanager
// tests use a fake reader, so only this test sees the production one.
func TestCopyVisibility_NamesHiddenFieldsWithoutValues(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(copyVisibilityMeta))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	policy, err := acl.ParsePolicy([]byte(copyVisibilityPolicy), "acl.yaml")
	if err != nil {
		t.Fatalf("ParsePolicy: %v", err)
	}
	d, err := acl.NewDeclarative(policy, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	st := memstore.New()
	withSalary := entity.New("PERS-1", "person")
	withSalary.SetString("name", "Alice")
	withSalary.SetString("salary", "99000")
	without := entity.New("PERS-2", "person")
	without.SetString("name", "Bob")
	for _, e := range []*entity.Entity{withSalary, without} {
		if cErr := st.CreateEntity(t.Context(), e); cErr != nil {
			t.Fatalf("CreateEntity: %v", cErr)
		}
	}
	tw, err := appbuild.CompileTransitions(meta, st, d)
	if err != nil {
		t.Fatalf("CompileTransitions: %v", err)
	}
	ctx := acltest.Bind(principal.With(t.Context(), principal.Principal{User: "bob", Tool: principal.ToolDataEntry}), t, d)
	for _, id := range []string{"PERS-1", "PERS-2"} {
		seen, ok, gErr := tw.Visibility.Get(ctx, "person", id, "")
		if gErr != nil || !ok {
			t.Fatalf("%s: Get = %v, %v; the row is readable", id, ok, gErr)
		}
		if !slices.Contains(seen.Redacted, "salary") {
			t.Errorf("%s: Redacted = %v, want salary named", id, seen.Redacted)
		}
		if _, has := seen.Properties["salary"]; has {
			t.Errorf("%s: the hidden value was not stripped", id)
		}
	}
}
