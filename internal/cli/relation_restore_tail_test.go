package cli

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// oneVersionHistory serves a single canned version for any key and records
// the key it was asked about.
type oneVersionHistory struct {
	store.VersionService
	asked []entity.RelationKey
}

func (h *oneVersionHistory) GetRelationVersion(
	_ context.Context, q store.RelationHistoryQuery, _ int,
) (*store.RelationVersionSnapshot, error) {
	h.asked = append(h.asked, q.Key)
	return &store.RelationVersionSnapshot{Content: "restored"}, nil
}

// recordingRelationWriter records which relation writes reach the manager.
type recordingRelationWriter struct {
	entityWriter
	creates, updates []entity.RelationKey
}

func (w *recordingRelationWriter) CreateRelation(
	_ context.Context, key entity.RelationKey, _ entity.RelationOptions,
) (*entity.Relation, error) {
	w.creates = append(w.creates, key)
	return &entity.Relation{}, nil
}

func (w *recordingRelationWriter) UpdateRelation(
	_ context.Context, key entity.RelationKey, _ entity.RelationOptions,
) (*entity.Relation, error) {
	w.updates = append(w.updates, key)
	return &entity.Relation{}, nil
}

// TestRelationRestore_FacedTail pins that `rela relation-restore ID@face`
// probes and writes the addressed tail (TKT-KQXVF7). Before, the liveness
// probe used the raw `ID@face` string as an id, missed the live edge, and
// the re-create went to the manager with that string as the source id and no
// face.
func TestRelationRestore_FacedTail(t *testing.T) {
	captureOut(t)
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft"},
		{ID: "CTL-1", Type: "control"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	k := entity.RelationKey{From: "POL-1", FromFace: "draft", Type: "cites", To: "CTL-1"}
	if _, err := st.CreateRelation(ctx, k, nil); err != nil {
		t.Fatal(err)
	}

	hist := &oneVersionHistory{}
	w := &recordingRelationWriter{}
	svc := &writeServices{readServices: readServices{Store: st, Versions: hist, World: store.TrivialScope()}, EntityManager: w}
	cmd := &RelationRestoreCmd{From: "POL-1@draft", Type: "cites", To: "CTL-1", Version: 1}
	if err := cmd.Run(ctx, svc); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(hist.asked) != 1 || hist.asked[0] != k {
		t.Errorf("history read %v, want [%s]", hist.asked, k)
	}
	if len(w.creates) != 0 || len(w.updates) != 1 || w.updates[0] != k {
		t.Errorf("creates %v, updates %v; want one update of %s", w.creates, w.updates, k)
	}
}
