package acl

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// facedRelationPolicy grants relation writes on a faced `policy` type. The
// drafter may create and update the draft face only; the editor holds both
// faces; the linker holds only the relation grant for `cites`.
const facedRelationPolicy = `
roles:
  drafter:
    read: ["*"]
    create: ["policy@draft"]
    update: ["policy@draft"]
  editor:
    read: ["*"]
    create: ["policy@draft", "policy@published"]
    update: ["policy@draft", "policy@published"]
  linker:
    read: ["*"]
    permissions: [link-cites]
  drafting-linker:
    read: ["*"]
    update: ["policy@draft"]
    permissions: [link-cites]
assignments:
  drafter: drafter
  editor: editor
  linker: linker
  drafting-linker: drafting-linker
relation_grants:
  cites:
    create: link-cites
`

// TestRelationWrite_IdentityEdgeUsesFamilyRule pins ruling D4: an
// identity-scoped edge from a faced source is authorized on every face in
// FamilyFaces, which the entity manager fills from the faces the type
// declares. The denial names no face, so it reads the same whichever faces
// the entity stores.
func TestRelationWrite_IdentityEdgeUsesFamilyRule(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, facedRelationPolicy)
	ctx := context.Background()
	family := RelationSubject{
		Type: "owns", FromType: "policy", FromID: "POL-1",
		FamilyFaces: []entity.Face{"draft", "published"},
	}
	draftOnly := RelationSubject{
		Type: "owns", FromType: "policy", FromID: "POL-1",
		FamilyFaces: []entity.Face{"draft"},
	}

	tests := []struct {
		name    string
		user    string
		subject RelationSubject
		allow   bool
	}{
		{"one face granted of two", "drafter", family, false},
		{"every face granted", "editor", family, true},
		{"family with only the granted face", "drafter", draftOnly, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mustRequest(t, d, tc.user).authorizeRelationWrite(ctx, OpCreate, tc.subject)
			if got.Allow != tc.allow {
				t.Fatalf("allow = %v, want %v (%s)", got.Allow, tc.allow, got.Reason)
			}
			if !tc.allow && (strings.Contains(got.Reason, "published") || strings.Contains(got.Reason, "draft")) {
				t.Errorf("denial %q names a face", got.Reason)
			}
		})
	}
}

// TestRelationGrant_DoesNotReachAnUnwritableTail pins the second half of D4: a
// relation grant does not satisfy a content-scoped edge on a face the
// principal cannot update.
func TestRelationGrant_DoesNotReachAnUnwritableTail(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, facedRelationPolicy)
	ctx := context.Background()
	cites := func(face entity.Face) RelationSubject {
		return RelationSubject{Type: "cites", FromType: "policy", FromID: "POL-1", FromFace: face}
	}

	tests := []struct {
		name    string
		user    string
		subject RelationSubject
		allow   bool
	}{
		{"grant alone, named tail", "linker", cites("draft"), false},
		{"grant alone, zero tail", "linker", cites(""), true},
		{"grant plus update on the tail", "drafting-linker", cites("draft"), true},
		{"grant plus update on another face", "drafting-linker", cites("published"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mustRequest(t, d, tc.user).authorizeRelationWrite(ctx, OpCreate, tc.subject)
			if got.Allow != tc.allow {
				t.Fatalf("allow = %v, want %v (%s)", got.Allow, tc.allow, got.Reason)
			}
			if tc.allow {
				return
			}
			if !strings.Contains(got.Reason, "which no role lets you update") {
				t.Errorf("denial %q does not say the relation grant stopped at the face", got.Reason)
			}
		})
	}
}
