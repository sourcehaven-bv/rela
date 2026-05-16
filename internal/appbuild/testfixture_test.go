package appbuild_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func parseTestMetamodel(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(metamodelYAML))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	return meta
}

func TestNewForTest_Defaults(t *testing.T) {
	meta := parseTestMetamodel(t)

	svc := appbuild.NewForTest(meta)
	if svc == nil {
		t.Fatal("NewForTest returned nil")
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

func TestNewForTest_WithTestStore(t *testing.T) {
	meta := parseTestMetamodel(t)
	customStore := memstore.New()

	svc := appbuild.NewForTest(meta, appbuild.WithTestStore(customStore))
	if svc.Store() != customStore {
		t.Error("WithTestStore did not install the supplied store")
	}
}

func TestNewForTest_NilMetaPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on nil metamodel, got none")
		}
	}()
	_ = appbuild.NewForTest(nil)
}
