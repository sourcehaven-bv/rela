package appbuildtest_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const metamodelYAML = `version: "1.0"
entities:
  item:
    label: Item
    plural: items
    id_prefix: "ITEM-"
    id_type: sequential
    properties:
      title:
        type: string
`

func parseTestMetamodel(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(metamodelYAML))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	return meta
}

func TestNew_Defaults(t *testing.T) {
	meta := parseTestMetamodel(t)

	svc := appbuildtest.New(meta)
	if svc == nil {
		t.Fatal("New returned nil")
	}
	if svc.Store() == nil {
		t.Error("Store() == nil")
	}
	if svc.Meta() != meta {
		t.Error("Meta() did not return the supplied metamodel")
	}
	if svc.EntityManager() == nil {
		t.Error("EntityManager() == nil")
	}
	if svc.Tracer() == nil {
		t.Error("Tracer() == nil")
	}
	if svc.ScriptEngine() == nil {
		t.Error("ScriptEngine() == nil")
	}
}

func TestNew_WithStore(t *testing.T) {
	meta := parseTestMetamodel(t)
	customStore := memstore.New()

	svc := appbuildtest.New(meta, appbuildtest.WithStore(customStore))
	if svc.Store() != customStore {
		t.Error("WithStore did not install the supplied store")
	}
}

func TestNew_NilMetaPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on nil metamodel, got none")
		}
	}()
	_ = appbuildtest.New(nil)
}

// RR-FGJR: WithDeclarative must wire both the ACL (entitymanager
// gets it as acl.ACL) and the concrete *acl.Declarative
// (svc.ACLDeclarative() returns non-nil for the affordance resolver).
func TestNew_WithDeclarative_WiresBothACLAndDeclarative(t *testing.T) {
	meta := parseTestMetamodel(t)
	d, err := acl.NewDeclarative(&acl.Policy{}, acl.NullGraph{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}

	svc := appbuildtest.New(meta, appbuildtest.WithDeclarative(d))
	if svc.ACL() != acl.ACL(d) {
		t.Errorf("svc.ACL() = %T, want the supplied *acl.Declarative", svc.ACL())
	}
	if svc.ACLDeclarative() != d {
		t.Errorf("svc.ACLDeclarative() = %p, want %p (the supplied Declarative)", svc.ACLDeclarative(), d)
	}
}
