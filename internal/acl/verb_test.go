package acl

import (
	"slices"
	"testing"
)

// TestVerbs_AreReadPlusOps pins the definition of Verb: read, then one verb per
// write Op, so a new Op cannot be missed by a consumer that enumerates verbs.
func TestVerbs_AreReadPlusOps(t *testing.T) {
	want := []Verb{VerbRead}
	for _, op := range Ops() {
		want = append(want, Verb(op))
	}
	if got := Verbs(); !slices.Equal(got, want) {
		t.Errorf("Verbs() = %v, want %v", got, want)
	}
}

func TestVerb_ValidAndOp(t *testing.T) {
	tests := []struct {
		verb   Verb
		valid  bool
		op     Op
		isOpOK bool
	}{
		{VerbRead, true, "", false},
		{VerbCreate, true, OpCreate, true},
		{VerbUpdate, true, OpUpdate, true},
		{VerbDelete, true, OpDelete, true},
		{VerbRename, true, OpRename, true},
		{"", false, "", false},
		{"Read", false, "", false},
		{"rename2", false, "", false},
	}
	for _, tc := range tests {
		t.Run(string(tc.verb), func(t *testing.T) {
			if got := tc.verb.Valid(); got != tc.valid {
				t.Errorf("Valid() = %v, want %v", got, tc.valid)
			}
			op, ok := tc.verb.op()
			if op != tc.op || ok != tc.isOpOK {
				t.Errorf("op() = (%q, %v), want (%q, %v)", op, ok, tc.op, tc.isOpOK)
			}
		})
	}
}

// TestOps_ReturnsACopy: a caller that edits the slice must not change the
// package's list.
func TestOps_ReturnsACopy(t *testing.T) {
	got := Ops()
	got[0] = "tampered"
	if Ops()[0] != OpCreate {
		t.Fatal("editing the result of Ops changed the next result")
	}
	if Op("tampered").Valid() {
		t.Fatal("editing the result of Ops changed Op.Valid")
	}
}
