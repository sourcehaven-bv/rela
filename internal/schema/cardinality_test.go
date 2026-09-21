package schema_test

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// failingListReader yields the iterator error before any row, so the scan
// collects nothing. Store must be non-nil: CountRelations delegates to it.
type failingListReader struct {
	store.Store
	err error
}

func (f failingListReader) ListEntities(
	context.Context, store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		yield(nil, f.err)
	}
}

// TestCheckCardinality_TruncatedScanIsLogged pins the error-policy ASYMMETRY
// that TKT-CICJSN's MCP copy got wrong in both directions.
//
// A failed relation COUNT propagates, because a count of 0 is
// indistinguishable from genuinely missing relations and would invent a
// violation (covered by the analysis package's count-error test). A truncated
// SUBJECT SCAN does not propagate, because it can only cause findings to be
// missed — but it must leave a trace, or a partial answer is indistinguishable
// from a clean graph. The deleted MCP copy did `if err != nil { break }` and
// left no trace at all.
//
// The assertion is on the log because the log IS the behavior here: the
// returned value is deliberately the same either way.
//
// No t.Parallel anywhere in this package, so swapping the default logger is
// safe; keep it that way if you add tests here.
func TestCheckCardinality_TruncatedScanAborts(t *testing.T) {
	one := 1
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket":  {Label: "Ticket", IDPrefixes: []string{"TKT-"}},
			"concept": {Label: "Concept", IDPrefixes: []string{"CON-"}},
		},
		Relations: map[string]metamodel.RelationDef{
			"affects": {From: []string{"ticket"}, To: []string{"concept"}, MinOutgoing: &one},
		},
	}

	st := memstore.New()
	ctx := context.Background()
	// Seeded but unreachable: the reader fails before yielding it. A scan
	// that silently truncated would report "no violations" for a graph whose
	// only ticket has no `affects` edge at all.
	if err := st.CreateEntity(ctx, &entity.Entity{ID: "TKT-001", Type: "ticket"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	scanErr := errors.New("scan interrupted")
	violations, err := schema.CheckCardinality(
		ctx, failingListReader{Store: st, err: scanErr}, meta, nil)
	if !errors.Is(err, scanErr) {
		t.Fatalf("a truncated scan must abort the run, got err=%v", err)
	}
	if len(violations) != 0 {
		t.Errorf("returned violations alongside a failed scan: %+v", violations)
	}
}
