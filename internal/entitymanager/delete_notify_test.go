package entitymanager_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// stateRecorder is an alias subscriber that also wants the deleted state.
type stateRecorder struct {
	recordingRewriter
	states []string // `id@face=title` per notification
}

func (r *stateRecorder) EntityStateDeleted(_ context.Context, last *entity.Entity) error {
	r.states = append(r.states, entity.FormatStateRef(last.ID, last.Face)+"="+last.GetString("title"))
	return r.deleteErr
}

// TestDeleteNotifier: every way to delete an entity reports each face it
// removed, with the face's last state, once; a purge and a restore report
// nothing (TKT-Q0AQ840).
func TestDeleteNotifier(t *testing.T) {
	seedFaces := func(t *testing.T, st *memstore.MemStore) {
		t.Helper()
		seedTask(t, st, "TSK-1", map[string]any{"title": "base"}, "")
		if err := st.CreateEntity(t.Context(), &entity.Entity{
			ID: "TSK-1", Type: "task", Face: "draft", Properties: map[string]any{"title": "draft"},
		}); err != nil {
			t.Fatalf("seed draft face: %v", err)
		}
	}
	tests := []struct {
		name string
		do   func(ctx context.Context, mgr *entitymanager.Manager) error
		want []string
	}{
		{"delete", func(ctx context.Context, mgr *entitymanager.Manager) error {
			_, err := mgr.DeleteEntity(ctx, "TSK-1", false)
			return err
		}, []string{"TSK-1=base", "TSK-1@draft=draft"}},
		{"face delete", func(ctx context.Context, mgr *entitymanager.Manager) error {
			_, err := mgr.DeleteEntityFace(ctx, "TSK-1", "draft", true)
			return err
		}, []string{"TSK-1@draft=draft"}},
		{"soft delete", func(ctx context.Context, mgr *entitymanager.Manager) error {
			_, err := entitymanager.SoftDeleteEntity(ctx, mgr, "TSK-1")
			return err
		}, []string{"TSK-1=base", "TSK-1@draft=draft"}},
		{"soft delete, restore, purge", func(ctx context.Context, mgr *entitymanager.Manager) error {
			if _, err := entitymanager.SoftDeleteEntity(ctx, mgr, "TSK-1"); err != nil {
				return err
			}
			if _, err := entitymanager.RestoreEntity(ctx, mgr, "TSK-1"); err != nil {
				return err
			}
			if _, err := entitymanager.SoftDeleteEntity(ctx, mgr, "TSK-1"); err != nil {
				return err
			}
			_, err := entitymanager.PurgeSoftDeleted(ctx, mgr, time.Now().Add(time.Hour))
			return err
		}, []string{"TSK-1=base", "TSK-1@draft=draft", "TSK-1=base", "TSK-1@draft=draft"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rw := &stateRecorder{}
			mgr, st := aliasHookManager(t, rw)
			seedFaces(t, st)
			if err := tc.do(t.Context(), mgr); err != nil {
				t.Fatal(err)
			}
			got := slices.Clone(rw.states)
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("notified %v, want %v", got, want)
			}
		})
	}
}
