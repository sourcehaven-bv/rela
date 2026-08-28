package store

import (
	"context"
	"errors"
)

// Derived-schema reconciliation (TKT-3Q0GP1).
//
// A "derived" schema object is a database constraint/index a backend CAN
// synthesize from the metamodel to enforce a declaration atomically that the
// application otherwise checks non-atomically. The motivating case is
// `unique: true`: [github.com/Sourcehaven-BV/rela/internal/entitymanager]
// enforces it with a check-then-write scan, which two concurrent (especially
// cross-process) writers can both pass. A store-level partial unique index
// closes that race — pgstore is the only backend that can, so this is an
// OPTIONAL capability type-asserted at the wiring site like [Formatter] /
// [HistoryReader], never part of [Store].
//
// Reconciliation is STATELESS desired-vs-actual convergence: the metamodel is
// the desired set, the backend's own catalog is the actual set, and a naming
// prefix marks ownership. There is no persisted "current state" record to drift
// from the reality — see [DerivedSchemaReconciler].

// DerivedObjectKind identifies which reconciler RULE produced a spec. Each rule
// owns a disjoint name sub-namespace in the backend catalog so rules never
// clobber one another's objects (the unique rule only ever drops its own
// indexes, a static-query rule only its own indexes, etc.).
type DerivedObjectKind string

const (
	// DerivedUnique is the `unique: true` rule: a partial unique index over a
	// single scalar property value, per entity type.
	DerivedUnique DerivedObjectKind = "unique"
	// DerivedQueryIndex accelerates one static scalar-equality query shape.
	DerivedQueryIndex DerivedObjectKind = "query-index"
)

// DerivedObjectSpec is one desired derived object, derived from validated
// project configuration. It is backend-agnostic: the backend translates it
// into concrete DDL. DerivedUnique uses Property; DerivedQueryIndex uses the
// canonical sorted Properties query shape.
type DerivedObjectSpec struct {
	Kind       DerivedObjectKind
	Type       string
	Property   string
	Properties []string
}

// DerivedObjectState is the outcome of reconciling one spec (or one discovered
// orphan object).
type DerivedObjectState string

const (
	// DerivedEnforced: the object already existed and matches desired — no-op.
	DerivedEnforced DerivedObjectState = "enforced"
	// DerivedCreated: the object was absent and was created this pass.
	DerivedCreated DerivedObjectState = "created"
	// DerivedDropped: the object existed, is no longer declared, and was
	// dropped this pass (the operator removed the declaration).
	DerivedDropped DerivedObjectState = "dropped"
	// DerivedUnenforced: the object is declared but could NOT be created —
	// almost always because pre-existing rows already violate it. This is
	// NON-fatal: the declaration is simply not enforced at the DB level (the
	// application-level check still runs). Reason carries a human string;
	// BlockingCount the number of offending value groups.
	DerivedUnenforced DerivedObjectState = "unenforced"
)

// DerivedObjectOutcome reports what a reconcile pass did (or a dry-run WOULD
// do) for one spec. SampleValues is populated only when the caller explicitly
// opts in (ReconcileOptions.ShowValues) — the blocking VALUES are entity
// content (secret), so by default only BlockingCount is surfaced. Surfacing
// this is an operator-shell affordance; it must never reach an API/health
// response (RR-3NB0P9).
type DerivedObjectOutcome struct {
	Spec          DerivedObjectSpec
	State         DerivedObjectState
	Reason        string
	BlockingCount int
	SampleValues  []string
	// WouldChange is true on a dry-run for any spec whose State is not
	// DerivedEnforced — i.e. reconcile would create, drop, or fail to enforce
	// it. Lets a CLI dry-run exit non-zero on drift without re-deriving.
	WouldChange bool
}

// ReconcileOptions controls a reconcile pass.
type ReconcileOptions struct {
	// DryRun computes the plan and reports outcomes WITHOUT issuing any DDL.
	// A dry-run outcome's State reflects what WOULD happen (created/dropped/
	// unenforced/enforced) and WouldChange flags drift.
	DryRun bool
	// ShowValues includes sample blocking values in DerivedUnenforced
	// outcomes. Operator-shell only; off by default.
	ShowValues bool
}

// DerivedSchemaReconciler is an OPTIONAL store capability (type-asserted like
// [HistoryReader]) that converges backend derived-schema objects toward the
// metamodel. A backend that cannot synthesize such objects (fs/mem) simply does
// not implement it; the wiring site degrades to the application-level check.
//
// Reconcile is idempotent: it introspects the live catalog each call, so a
// second identical call is a no-op and a hand-dropped object is recreated. It
// must be safe to call at every store-open and on demand from the CLI. It is
// the ONE planner behind store-open reconcile, `rela db status`, and `rela db
// reconcile [--dry-run]` so a dry-run prediction cannot drift from what
// store-open actually does.
type DerivedSchemaReconciler interface {
	// Reconcile converges the derived schema for the desired specs and returns
	// one outcome per desired spec plus one per discovered orphan (an owned
	// object no longer desired). It returns an error only for an infrastructure
	// failure (e.g. the catalog is unreadable); a single object that cannot be
	// enforced is reported as a DerivedUnenforced outcome, NOT an error — a
	// derived-schema problem must never fail store-open.
	Reconcile(ctx context.Context, desired []DerivedObjectSpec, opts ReconcileOptions) ([]DerivedObjectOutcome, error)
}

// UniquePropertyError is returned by a write when a store-level derived unique
// index rejects it (as opposed to [ErrConflict], which is an entity-ID clash).
// It names the property so [github.com/Sourcehaven-BV/rela/internal/entitymanager]
// can map it to the SAME property-level validation error its own check-then-write
// scan produces — the two enforcement paths are indistinguishable to a client.
// Property is empty when the backend could not attribute the violation to a
// known property (e.g. a rolling deploy against an index a newer peer created);
// callers treat an empty-Property UniquePropertyError as a generic conflict, never
// a 500 (RR-B5Y6DZ).
type UniquePropertyError struct {
	Property string
}

func (e UniquePropertyError) Error() string {
	if e.Property == "" {
		return "store: unique property constraint violated"
	}
	return "store: unique property constraint violated: " + e.Property
}

// Is reports UniquePropertyError as an ErrConflict for callers that only care
// that the write lost a uniqueness race, so existing `errors.Is(err,
// ErrConflict)` sites keep working unchanged while property-aware callers can
// still `errors.As` the richer error.
func (e UniquePropertyError) Is(target error) bool {
	return errors.Is(target, ErrConflict)
}
