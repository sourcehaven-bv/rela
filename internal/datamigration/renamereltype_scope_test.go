package datamigration

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestRenameRelationType_RefusesScopeChange pins that a rename between a
// content-scoped and an identity-scoped type is refused: the step copies
// each edge's tail verbatim, so the new type would hold tails it does not
// expect.
func TestRenameRelationType_RefusesScopeChange(t *testing.T) {
	shape := func(name string, scope metamodel.RelationScope) metamodel.ShapeProjection {
		return metamodel.ShapeProjection{Relations: map[string]metamodel.RelationShape{
			name: {From: []string{"page"}, To: []string{"spec"}, Scope: scope},
		}}
	}
	tests := []struct {
		name          string
		fromScope     metamodel.RelationScope
		toScope       metamodel.RelationScope
		wantRefusal   bool
		wantInMessage string
	}{
		{"content to identity", metamodel.ScopeContent, "", true, "content-scoped"},
		{"identity to content", "", metamodel.ScopeContent, true, "identity-scoped"},
		{"content to content", metamodel.ScopeContent, metamodel.ScopeContent, false, ""},
		{"identity to identity", "", "", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			step := &renameRelationTypeStep{From: "cites", To: "refs"}
			err := step.Validate(shape("cites", tc.fromScope), shape("refs", tc.toScope))
			if !tc.wantRefusal {
				if err != nil {
					t.Fatalf("Validate: unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate accepted a rename across relation scopes")
			}
			for _, want := range []string{tc.wantInMessage, "data migration", "tail"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}
