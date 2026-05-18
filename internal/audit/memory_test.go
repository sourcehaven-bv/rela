package audit_test

import (
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
)

func TestMemory_RecordsAccumulate(t *testing.T) {
	m := audit.NewMemory()
	m.Record(audit.Record{Op: audit.OpCreateEntity})
	m.Record(audit.Record{Op: audit.OpDeleteEntity})

	got := m.Records()
	if len(got) != 2 {
		t.Fatalf("want 2 records, got %d", len(got))
	}
	if got[0].Op != audit.OpCreateEntity || got[1].Op != audit.OpDeleteEntity {
		t.Errorf("unexpected ops: %+v", got)
	}
}

func TestMemory_RecordsIsSnapshot(t *testing.T) {
	m := audit.NewMemory()
	m.Record(audit.Record{Op: audit.OpCreateEntity})

	snap := m.Records()
	// Mutating the snapshot must not affect the backend.
	snap[0].Op = "tampered"

	got := m.Records()
	if got[0].Op != audit.OpCreateEntity {
		t.Errorf("backend was mutated through snapshot: got %q", got[0].Op)
	}

	// Adding a record after the snapshot must not appear in the snapshot.
	m.Record(audit.Record{Op: audit.OpDeleteEntity})
	if len(snap) != 1 {
		t.Errorf("snapshot grew: len=%d", len(snap))
	}
}

func TestMemory_ConcurrentRecord(t *testing.T) {
	m := audit.NewMemory()
	var wg sync.WaitGroup
	const n = 100
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Record(audit.Record{Op: audit.OpCreateEntity})
		}()
	}
	wg.Wait()
	if len(m.Records()) != n {
		t.Errorf("want %d records, got %d", n, len(m.Records()))
	}
}
