package tracer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// TestTraceFrom_IgnoresQueryScopes is AC6 for the tracer (TKT-EVR2TU).
//
// A query scope is a PRESENTATION narrowing: it decides what a person's screen
// shows. The tracer answers what is TRUE about the graph, which is a different
// question, and the two must not be confused — a trace that silently omitted
// scope-excluded nodes would report a requirement as unimplemented because the
// implementing decision happened to be archived.
//
// Here the property is STRUCTURAL rather than merely untested: [tracer.New]
// takes a narrow reader (store.EntityReader + store.RelationReader) and the
// tracer holds no metamodel at all, so there is nowhere for a `query_scopes:`
// declaration to reach it. This test pins the observable consequence — an
// archived node is still traced — so that a future refactor threading a
// metamodel through here has to delete an assertion that says why not to.
func TestTraceFrom_IgnoresQueryScopes(t *testing.T) {
	s := memstore.New()

	live := entity.New("DEC-1", "decision")
	live.SetString("title", "Live decision")
	live.SetString("status", "open")
	require.NoError(t, s.CreateEntity(t.Context(), live))

	// The row a `default: "entity.status ~= 'gearchiveerd'"` scope would hide
	// on every data-entry surface.
	archived := entity.New("REQ-1", "requirement")
	archived.SetString("title", "Archived requirement")
	archived.SetString("status", "gearchiveerd")
	require.NoError(t, s.CreateEntity(t.Context(), archived))

	_, err := s.CreateRelation(t.Context(), "DEC-1", "implements", "REQ-1", nil)
	require.NoError(t, err)

	result := tracer.New(s).TraceFrom(t.Context(), "DEC-1", 0)
	require.NotNil(t, result)

	if !tracedIDs(result)["REQ-1"] {
		t.Fatalf("archived node missing from trace; the tracer must answer what IS, "+
			"not what a screen shows. Traced: %v", tracedIDs(result))
	}
}

// TestFindOrphans_IgnoresQueryScopes is AC6 for the orphan report.
//
// This is the ticket's named failure mode in miniature: an orphan report that
// honored a default scope would stop counting archived rows as orphans, so a
// genuinely unlinked entity would disappear from the report by being archived
// — the analysis reports clean over data it was never shown.
func TestFindOrphans_IgnoresQueryScopes(t *testing.T) {
	s := memstore.New()

	orphan := entity.New("REQ-9", "requirement")
	orphan.SetString("title", "Archived orphan")
	orphan.SetString("status", "gearchiveerd")
	require.NoError(t, s.CreateEntity(t.Context(), orphan))

	orphans, err := tracer.New(s).FindOrphans(t.Context())
	require.NoError(t, err)

	found := false
	for _, id := range orphans {
		if id == "REQ-9" {
			found = true
		}
	}
	if !found {
		t.Fatalf("archived orphan missing from report; scoping this read would let "+
			"archiving an entity hide it from the orphan check. Got: %v", orphans)
	}
}

// tracedIDs flattens a trace result into the set of ids it reached.
func tracedIDs(r *tracer.TraceResult) map[string]bool {
	out := map[string]bool{}
	var walk func(n *tracer.TraceResult)
	walk = func(n *tracer.TraceResult) {
		if n == nil {
			return
		}
		out[n.ID] = true
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(r)
	return out
}
