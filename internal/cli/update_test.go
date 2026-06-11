package cli

import (
	"reflect"
	"testing"
)

// update_test.go covers only CLI-level concerns. The property flag
// parsing itself is verified by TestParsePropertyFlag in create_test.go
// (shared helper), and entity property mutation is covered by the
// entity package tests — no need to duplicate either here.

func TestUpdateCmd_PropertyFlagExists(t *testing.T) {
	rt := reflect.TypeOf(UpdateCmd{})
	f, ok := rt.FieldByName("Property")
	if !ok {
		t.Fatal("update command struct should have a Property field")
	}
	if got := f.Tag.Get("short"); got != "P" {
		t.Errorf("Property field short tag = %q, want %q", got, "P")
	}
}
