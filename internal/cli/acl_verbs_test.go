package cli

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/aclmap"
)

// TestACLVerbEnumsMatchReportVerbs pins the `--verb` enums of the access
// reports to aclmap.Verbs, which follows acl.Ops. A struct tag cannot call a
// function, so this test is what makes a new write Op reach `rela acl can`,
// `who-can` and `map`. `acl can-relation` is not listed: a relation has its
// own verb set (aclmap.RelationVerbValid).
func TestACLVerbEnumsMatchReportVerbs(t *testing.T) {
	want := make([]string, 0, len(aclmap.Verbs()))
	for _, v := range aclmap.Verbs() {
		want = append(want, string(v))
	}

	tests := []struct {
		name     string
		cmd      any
		optional bool // the map filter also accepts "" for "every verb"
	}{
		{name: "acl can", cmd: ACLCanCmd{}},
		{name: "acl who-can", cmd: ACLWhoCanCmd{}},
		{name: "acl map", cmd: ACLMapCmd{}, optional: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			field, ok := reflect.TypeOf(tc.cmd).FieldByName("Verb")
			if !ok {
				t.Fatal("command has no Verb field")
			}
			got := strings.Split(field.Tag.Get("enum"), ",")
			wantHere := want
			if tc.optional {
				wantHere = append([]string{""}, want...)
			}
			if !slices.Equal(got, wantHere) {
				t.Errorf("enum = %q, want %q", got, wantHere)
			}
		})
	}
}
