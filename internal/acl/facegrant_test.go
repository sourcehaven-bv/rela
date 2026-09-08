package acl

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// A read grant may name a content state (TKT-O7R2A1). These pin the grant
// vocabulary; the end-to-end gating is pinned in internal/dataentry.
func TestRoleReadFaces(t *testing.T) {
	for _, tc := range []struct {
		name      string
		read      []string
		target    string
		wantAll   bool
		wantFaces []entity.Face
	}{
		{
			// The asymmetry with the write side, deliberate: a bare read grant
			// covers every face. Narrowing it would mean a bare grant under a
			// world that resolves to `published` reads NOTHING, since a world
			// never serves the default face — a total outage, not a narrowing.
			name: "bare type grant reads every face",
			read: []string{"policy"}, target: "policy", wantAll: true,
		},
		{
			name: "wildcard reads every face",
			read: []string{"*"}, target: "policy", wantAll: true,
		},
		{
			name: "face grant narrows to that face",
			read: []string{"policy@published"}, target: "policy",
			wantFaces: []entity.Face{"published"},
		},
		{
			// Grants are additive (DEC-RG878), so two face grants union.
			name: "two face grants union",
			read: []string{"policy@published", "policy@review"}, target: "policy",
			wantFaces: []entity.Face{"published", "review"},
		},
		{
			// A bare grant beside a face grant widens to everything: the bare
			// one already covers every face, so the union cannot be narrower.
			name: "bare grant beside a face grant wins",
			read: []string{"policy@published", "policy"}, target: "policy",
			wantAll: true,
		},
		{
			name: "another type's face grant does not apply",
			read: []string{"guide@nl"}, target: "policy",
		},
		{
			// World grants share the Read list and address worlds, not types.
			name: "world grants are ignored",
			read: []string{"world:published"}, target: "policy",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			faces, all := roleReadFaces(RoleDef{Read: tc.read}, tc.target)
			if all != tc.wantAll {
				t.Fatalf("all = %v, want %v", all, tc.wantAll)
			}
			if all {
				return
			}
			if len(faces) != len(tc.wantFaces) {
				t.Fatalf("faces = %v, want %v", faces, tc.wantFaces)
			}
			got := map[entity.Face]bool{}
			for _, f := range faces {
				got[f] = true
			}
			for _, w := range tc.wantFaces {
				if !got[w] {
					t.Errorf("missing face %q in %v", w, faces)
				}
			}
		})
	}
}

// A face grant must still compose a query: without this the role would read
// nothing at all, because roleGrantsRead would not recognize the type.
func TestFaceGrantStillGrantsTheType(t *testing.T) {
	if !roleGrantsRead(RoleDef{Read: []string{"policy@published"}}, "policy") {
		t.Fatal("a face-scoped grant must grant its TYPE for query composition; " +
			"otherwise the role composes no query and reads nothing")
	}
}

// The set must be stable across calls: a query whose arguments reshuffle
// between identical requests defeats statement caching and is unreadable in a
// diff.
func TestDedupeFacesIsStableAndDeduped(t *testing.T) {
	got := dedupeFaces([]entity.Face{"b", "a", "b", ""})
	want := []entity.Face{"", "a", "b"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
