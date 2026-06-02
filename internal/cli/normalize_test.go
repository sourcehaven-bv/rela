package cli

import (
	"reflect"
	"testing"
)

func TestNormalizeCmd_DryRunFlagExists(t *testing.T) {
	rt := reflect.TypeOf(NormalizeCmd{})
	f, ok := rt.FieldByName("DryRun")
	if !ok {
		t.Fatal("normalize command struct should have a DryRun field")
	}
	if got := f.Tag.Get("name"); got != "dry-run" {
		t.Errorf("DryRun field name tag = %q, want %q", got, "dry-run")
	}
}
