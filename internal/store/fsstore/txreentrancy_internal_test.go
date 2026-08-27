package fsstore

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestTx_ReadsViaOuterHandleDoNotDeadlock pins the property TKT-8HDPQW's
// design depends on, and which the package comment states but nothing
// asserted: "Readers never take it [txMu]" (fsstore.go).
//
// The cascade-delete gate authorizes INSIDE Tx, and the ACL evaluator reaches
// the graph through acl.StoreGraph — which holds the OUTER store handle and
// calls GetRelation / ListRelations on it. The tx.go note that "calling a
// write on the OUTER store from inside fn deadlocks" makes the read case worth
// pinning explicitly: if a future change gave readers txMu, authorization
// inside Tx would hang the whole server rather than fail a test somewhere
// obvious.
func TestTx_ReadsViaOuterHandleDoNotDeadlock(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	if err := s.CreateEntity(ctx, entity.New("REQ-1", "requirement")); err != nil {
		t.Fatalf("seed entity: %v", err)
	}
	if err := s.CreateEntity(ctx, entity.New("SOL-1", "solution")); err != nil {
		t.Fatalf("seed entity: %v", err)
	}
	if _, err := s.CreateRelation(ctx, "REQ-1", "satisfied-by", "SOL-1", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- s.Tx(ctx, func(tx store.Store) error {
			// Exactly what acl.StoreGraph does: reads against the OUTER
			// handle, not the tx view.
			if _, err := s.GetEntity(ctx, "REQ-1"); err != nil {
				return err
			}
			if _, err := s.GetRelation(ctx, "REQ-1", "satisfied-by", "SOL-1"); err != nil {
				return err
			}
			for _, err := range s.ListRelations(ctx, store.RelationQuery{
				EntityID: "REQ-1", Direction: store.DirectionOutgoing,
			}) {
				if err != nil {
					return err
				}
			}
			// The write still goes through the tx view, per the contract.
			_, err := tx.DeleteEntity(ctx, "SOL-1", true)
			return err
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Tx callback: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("DEADLOCK: a read via the outer handle inside Tx did not return — " +
			"readers have taken txMu, which breaks authorize-inside-Tx")
	}
}
