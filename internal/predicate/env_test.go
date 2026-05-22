package predicate_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

func TestDeclareFunc_RejectsRecordReturn(t *testing.T) {
	env := predicate.NewEnv()
	err := env.DeclareFunc("bad", predicate.FuncSig{
		Return: predicate.RecordType{"status": predicate.StringType},
	})
	if err == nil {
		t.Fatal("expected DeclareFunc to reject record return type")
	}
	if !strings.Contains(err.Error(), "record return") {
		t.Fatalf("unexpected reason: %v", err)
	}
}

func TestDeclareFunc_RejectsListReturn(t *testing.T) {
	env := predicate.NewEnv()
	err := env.DeclareFunc("bad", predicate.FuncSig{
		Return: predicate.ListType{Elem: predicate.StringType},
	})
	if err == nil {
		t.Fatal("expected DeclareFunc to reject list return type")
	}
	if !strings.Contains(err.Error(), "list return") {
		t.Fatalf("unexpected reason: %v", err)
	}
}

func TestBindings_RejectsEmptyName(t *testing.T) {
	b := predicate.NewBindings()
	if err := b.SetVar("", predicate.NewBool(true)); err == nil {
		t.Fatal("SetVar with empty name should error")
	}
	if err := b.SetFunc("", nil); err == nil {
		t.Fatal("SetFunc with empty name should error")
	}
}

func TestBindings_RejectsNilValue(t *testing.T) {
	b := predicate.NewBindings()
	if err := b.SetVar("x", nil); err == nil {
		t.Fatal("SetVar with nil value should error")
	}
}

func TestBindings_RejectsNilFunc(t *testing.T) {
	b := predicate.NewBindings()
	if err := b.SetFunc("f", nil); err == nil {
		t.Fatal("SetFunc with nil impl should error")
	}
}
