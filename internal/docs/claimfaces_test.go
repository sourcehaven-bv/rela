package docs

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestClaimFaces pins which faces an authorization claim is about
// (BUG-GJUBSA): a rename, and a delete naming no face, cover every declared
// face, as the manager authorizes them.
func TestClaimFaces(t *testing.T) {
	t.Parallel()
	declared := []string{"draft", "published"}
	all := []entity.Face{"draft", "published"}
	tests := []struct {
		name     string
		op       acl.Op
		face     string
		declared []string
		want     []entity.Face
		wantFail bool
	}{
		{name: "faceless", op: acl.OpUpdate, want: []entity.Face{entity.ImplicitFace}},
		{name: "faceless refuses a face", op: acl.OpUpdate, face: "draft", wantFail: true},
		{name: "update names its face", op: acl.OpUpdate, face: "draft", declared: declared,
			want: []entity.Face{"draft"}},
		{name: "update needs a face", op: acl.OpUpdate, declared: declared, wantFail: true},
		{name: "rename is the family", op: acl.OpRename, declared: declared, want: all},
		{name: "rename refuses a face", op: acl.OpRename, face: "draft", declared: declared, wantFail: true},
		{name: "bare delete is the family", op: acl.OpDelete, declared: declared, want: all},
		{name: "face delete", op: acl.OpDelete, face: "published", declared: declared,
			want: []entity.Face{"published"}},
		{name: "undeclared face", op: acl.OpUpdate, face: "review", declared: declared, wantFail: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, msg := claimFaces("allows", tc.op, "policy", tc.face, tc.declared)
			if tc.wantFail != (msg != "") {
				t.Fatalf("claimFaces failure = %q, want failure %v", msg, tc.wantFail)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("claimFaces = %v, want %v", got, tc.want)
			}
		})
	}
}
