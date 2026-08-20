package memstore

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// recordingObserver records the ids EntityPut delivers.
type recordingObserver struct {
	puts    []string
	deletes []string
}

func (r *recordingObserver) EntityPut(e *entity.Entity) error {
	r.puts = append(r.puts, e.ID+"|"+string(e.Pointer))
	return nil
}
func (r *recordingObserver) EntityDelete(id string) error {
	r.deletes = append(r.deletes, id)
	return nil
}
func (r *recordingObserver) EntityRenamed(string, *entity.Entity) error { return nil }

// TestObservers_SkipNonDefaultStates pins the Step-1 observer contract
// (TKT-DOFYR1, RR-8U1PE2): observers — the search indexers — key
// documents by bare id, so a non-default state write must NOT reach
// them or it would overwrite the default face in the index. The skip
// lives at the notify site; Subscribe() events are unfiltered.
// TODO(step 5 / TKT-9KZGJO): revisit when per-world indexing lands.
func TestObservers_SkipNonDefaultStates(t *testing.T) {
	obs := &recordingObserver{}
	m := New(WithObserver(obs))
	ctx := context.Background()

	def := entity.New("PAGE-1", "page")
	if err := m.CreateEntity(ctx, def); err != nil {
		t.Fatal(err)
	}
	draft := entity.New("PAGE-1", "page")
	p, err := entity.ParsePointer("draft")
	if err != nil {
		t.Fatal(err)
	}
	draft.Pointer = p
	if err := m.CreateEntity(ctx, draft); err != nil {
		t.Fatal(err)
	}
	draft.SetString("title", "edited")
	if err := m.UpdateEntity(ctx, draft); err != nil {
		t.Fatal(err)
	}

	want := []string{"PAGE-1|"}
	if len(obs.puts) != 1 || obs.puts[0] != want[0] {
		t.Errorf("observer puts = %v, want only the default state %v", obs.puts, want)
	}
}
