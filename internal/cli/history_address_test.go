package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// fakeFaceHistory answers ListStateVersions from a map keyed by the state
// address (`ID` or `ID@face`). An absent key is an empty lineage.
type fakeFaceHistory map[string]int

func (f fakeFaceHistory) ListStateVersions(_ context.Context, id string, face entity.Face) ([]store.VersionMeta, error) {
	n := f[entity.FormatStateRef(id, face)]
	metas := make([]store.VersionMeta, n)
	for i := range metas {
		metas[i] = store.VersionMeta{Version: i + 1}
	}
	return metas, nil
}

func (f fakeFaceHistory) GetStateVersion(
	context.Context, string, entity.Face, int,
) (*store.VersionSnapshot, error) {
	return nil, store.ErrNotFound
}

// TestHistoryAddress pins BUG-4SYAA6 at the CLI: a bare id resolves to the
// zero face only when that lineage is the entity's, and a bare id of a faced
// entity is refused with the faces named, live or deleted.
func TestHistoryAddress(t *testing.T) {
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"ticket": {Faces: map[string]metamodel.FaceDef{"draft": {}, "published": {}}},
		"note":   {},
	}}
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Face: "draft"},
		{ID: "TKT-1", Type: "ticket", Face: "published"},
		{ID: "NOTE-1", Type: "note"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	history := fakeFaceHistory{
		"TKT-1@draft": 2, "TKT-1@published": 1,
		"TKT-2@draft": 1, // deleted faced entity: history only
		"NOTE-2":      1, // deleted unfaced entity
	}

	tests := []struct {
		name    string
		raw     string
		want    entity.Ref
		wantErr []string
	}{
		{name: "explicit face passes", raw: "TKT-1@draft", want: entity.Ref{ID: "TKT-1", Face: "draft"}},
		{name: "unfaced live id passes", raw: "NOTE-1", want: entity.Ref{ID: "NOTE-1"}},
		{name: "unfaced deleted id passes", raw: "NOTE-2", want: entity.Ref{ID: "NOTE-2"}},
		{name: "unknown id passes as empty lineage", raw: "NOPE-1", want: entity.Ref{ID: "NOPE-1"}},
		{
			name: "bare live faced id is refused", raw: "TKT-1",
			wantErr: []string{"TKT-1@draft, TKT-1@published"},
		},
		{
			name: "bare deleted faced id is refused", raw: "TKT-2",
			wantErr: []string{"TKT-2@draft"},
		},
		{name: "malformed address", raw: "TKT-1@", wantErr: []string{"invalid entity address"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := historyAddress(ctx, st, meta, history, tc.raw)
			if len(tc.wantErr) > 0 {
				if err == nil {
					t.Fatalf("historyAddress(%q) = %v, want an error", tc.raw, got)
				}
				for _, want := range tc.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not name %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("historyAddress(%q): %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("historyAddress(%q) = %+v, want %+v", tc.raw, got, tc.want)
			}
		})
	}
}
