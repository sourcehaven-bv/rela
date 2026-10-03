package dataentry

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestRelationSources_ReadsTheTailFace pins which rows gate an incoming edge's
// write. The source's row at the edge's tail decides when it exists. A faced
// source has no zero-face row (DEC-NPZICR), so a new or identity-scoped edge
// from one is judged against every face of its family, never against the path
// entity, whose type's policy is the wrong one.
func TestRelationSources_ReadsTheTailFace(t *testing.T) {
	app := buildPolicyApp(t, relationRedactionACL, nil)
	ctx := t.Context()
	for _, face := range []entity.Face{"draft", "published"} {
		peer := &entity.Entity{ID: "TKT-F", Type: "ticket", Face: face, Properties: map[string]any{"title": "d"}}
		if err := app.store.CreateEntity(ctx, peer); err != nil {
			t.Fatal(err)
		}
	}
	path := &entity.Entity{ID: "PATH-1", Type: "path"}
	svc := app.affordances
	incoming := string(DirectionIncoming)

	faces := func(t *testing.T, peer entity.Ref, direction string) []string {
		t.Helper()
		got, err := svc.relationSources(ctx, path, peer, direction)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(got))
		for i, src := range got {
			out[i] = src.row.ID + "@" + string(src.row.Face)
			if src.unconditional {
				out[i] += " unconditional"
			}
		}
		return out
	}

	tests := []struct {
		name      string
		peer      entity.Ref
		direction string
		want      []string
	}{
		{"tail row", entity.Ref{ID: "TKT-F", Face: "draft"}, incoming, []string{"TKT-F@draft"}},
		{"zero-face tail of a faced peer", entity.Ref{ID: "TKT-F", Face: entity.ImplicitFace}, incoming,
			[]string{"TKT-F@draft", "TKT-F@published"}},
		{"missing peer", entity.Ref{ID: "TKT-NONE", Face: entity.ImplicitFace}, incoming, []string{"PATH-1@"}},
		{"outgoing", entity.Ref{ID: "TKT-F", Face: "draft"}, "outgoing", []string{"PATH-1@"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := faces(t, tc.peer, tc.direction)
			if len(got) != len(tc.want) {
				t.Fatalf("sources = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("sources = %v, want %v", got, tc.want)
				}
			}
		})
	}

	t.Run("family read fault", func(t *testing.T) {
		faulty := svc
		fault := errors.New("store down")
		faulty.sourceFamily = func(context.Context, string) ([]*entity.Entity, error) { return nil, fault }
		if _, err := faulty.relationSources(ctx, path, entity.Ref{ID: "TKT-F", Face: entity.ImplicitFace}, incoming); !errors.Is(err, fault) {
			t.Errorf("err = %v, want the read fault", err)
		}
	})
}
