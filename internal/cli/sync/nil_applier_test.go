package sync

import (
	"context"
	"errors"
	"testing"
)

// TestRecordCreate_NilApplier_ErrorsNotPanics pins the fix for the contract lie
// in NewEngine's godoc (TKT-IVSJV6).
//
// The doc used to say "applier may be nil for a push-only run (push never
// writes locally)". Push DOES write locally when the primary mints an id that
// differs from the local temp id: adopting it is a local rename. Pull and force
// guarded that with a nil check; recordCreate dereferenced blind, so a nil
// applier there was a panic on the ORDINARY create path, not a degraded run.
//
// The panic was unreachable while the CLI held a concrete *entitymanager.Manager.
// Narrowing that field to an interface made it reachable, which is why the guard
// landed with this ticket.
func TestRecordCreate_NilApplier_ErrorsNotPanics(t *testing.T) {
	h := newHarness(t)

	// Same client/store/index as the harness, but NO applier.
	noApplier, err := NewEngine(h.engine.client, h.st, nil, h.idx)
	if err != nil {
		t.Fatalf("NewEngine with nil applier: %v", err)
	}

	// Applied, with a CreatedID differing from the local key: the id-adoption
	// path, and the exact branch that used to deref a nil applier.
	_, err = noApplier.recordCreate(
		context.Background(),
		LocalChange{Key: "TKT-temp1"},
		&PushResult{Applied: true, CreatedID: "TKT-minted1"},
	)
	if err == nil {
		t.Fatal("recordCreate with a nil applier on the id-adoption path should " +
			"error; it must never dereference the applier blind")
	}
	if !errors.Is(err, errLocalApplierRequired) {
		t.Fatalf("want errLocalApplierRequired, got %v", err)
	}
}
