package visibility

import (
	"context"
	"errors"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// ErrReaderUnavailable is returned by [DenyReader] for every read. It
// signals that a gate was REQUIRED but could not be constructed — never
// that the caller lacks permission.
var ErrReaderUnavailable = errors.New("visibility: read gate unavailable; refusing to read ungated")

// DenyReader refuses every read. It is the fail-closed substitute for a
// script read handle when an ACL policy IS configured but the gate could
// not be built (RR-GKCZO5).
//
// Why this exists: degrading to the raw store on a construction fault is
// defensible for an interactive request — the operator sees the failure
// immediately and a broken gate breaking every request is the louder,
// safer-to-notice outage. It is NOT defensible for an unattended job. A
// nightly task that quietly reverts to reading the whole graph, builds a
// prompt from it, and ships it to a third-party model produces an
// unbounded, silent, irreversible disclosure — nobody reads the warning
// until long after the data has left. For that path, refusing to run is
// strictly better than running unconfined.
//
// Reads return [ErrReaderUnavailable] rather than a not-found, so the
// failure is diagnosable and never mistaken for "no such entity".
type DenyReader struct{}

// GetEntity implements the script read surface: always refuses.
func (DenyReader) GetEntity(context.Context, string) (*entity.Entity, error) {
	return nil, ErrReaderUnavailable
}

// ListEntities implements the script read surface: always refuses.
func (DenyReader) ListEntities(
	context.Context, store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		yield(nil, ErrReaderUnavailable)
	}
}

// ListRelations implements the script read surface: always refuses.
func (DenyReader) ListRelations(
	context.Context, store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return func(yield func(*entity.Relation, error) bool) {
		yield(nil, ErrReaderUnavailable)
	}
}

// DenyTracer refuses every traversal, the [DenyReader] counterpart for the
// tracer handle. Same rationale (RR-GKCZO5): when a policy is configured
// but the decorator cannot be built, an unattended job must not silently
// traverse the whole graph.
//
// tracer.Tracer's methods return no error, so refusal is expressed as the
// empty result each method already uses for "nothing found".
//
// CAVEAT — refusal is INVISIBLE to a script. The three traversals bound to
// Lua (rela.trace_from, rela.trace_to, rela.find_path) are exactly the three
// that cannot report failure, and a nil result is the same value a script
// gets for a nonexistent ID. FindOrphans does return an error, but no script
// can call it. So the `slog.Error` at the WIRING SITE is the only signal
// that a traversal was refused rather than genuinely empty — correlate by
// timestamp when a script reports an unexpectedly empty graph.
//
// This is accepted deliberately: seeing less than the truth is the safe
// direction, and the alternative (traversing ungated) is the disclosure this
// type exists to prevent. Surfacing it properly needs an error-returning
// traversal on tracer.Tracer — an interface change, tracked separately.
type DenyTracer struct{}

// TraceFrom implements [tracer.Tracer]: always empty.
func (DenyTracer) TraceFrom(context.Context, string, int) *tracer.TraceResult { return nil }

// TraceTo implements [tracer.Tracer]: always empty.
func (DenyTracer) TraceTo(context.Context, string, int) *tracer.TraceResult { return nil }

// FindPath implements [tracer.Tracer]: always empty.
func (DenyTracer) FindPath(context.Context, string, string) []tracer.PathStep { return nil }

// FindOrphans implements [tracer.Tracer]: refuses with an error, since this
// one CAN report failure rather than looking like an empty graph.
func (DenyTracer) FindOrphans(context.Context) ([]string, error) {
	return nil, ErrReaderUnavailable
}

// HasCycle implements [tracer.Tracer]: always false.
func (DenyTracer) HasCycle(context.Context, string) bool { return false }
