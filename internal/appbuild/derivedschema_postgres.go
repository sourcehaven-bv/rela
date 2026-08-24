//go:build postgres

package appbuild

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// derivedSchemaReconciler is the capability reconcileDerivedSchemaIfSupported
// needs: a store that can synthesize derived schema objects (partial unique
// indexes) from the metamodel and accept the unique-spec list used to attribute
// a violation to a property.
//
// Declared at the call site rather than beside the implementation, so any store
// offering these methods is discovered — not just one concrete backend
// (TKT-415WA7). Both methods are taken together deliberately: publishing specs
// without reconciling, or the reverse, is never wanted.
type derivedSchemaReconciler interface {
	SetUniqueSpecProvider(specs []store.DerivedObjectSpec)
	Reconcile(
		ctx context.Context, desired []store.DerivedObjectSpec, opts store.ReconcileOptions,
	) ([]store.DerivedObjectOutcome, error)
}

// reconcileDerivedSchemaIfSupported converges the derived schema at store-open
// and publishes the metamodel's unique (type, property) pairs so the write path
// can attribute a unique-index violation to a property (TKT-3Q0GP1). A store
// without the capability (should not happen in this build) is skipped. Reconcile
// failures are logged and swallowed: a derived-schema problem — most often
// pre-existing duplicate values blocking an index — must NEVER fail store-open.
// An operator inspects/repairs drift via `rela db status` / `rela db reconcile`.
func reconcileDerivedSchemaIfSupported(ctx context.Context, st store.Store, meta *metamodel.Metamodel) {
	s, ok := st.(derivedSchemaReconciler)
	if !ok {
		return
	}

	specs := uniqueSpecsFromMetamodel(meta)

	// Publish the pairs first so that even if the reconcile below degrades, a
	// violation of an already-present index still maps to its property.
	s.SetUniqueSpecProvider(specs)

	outcomes, err := s.Reconcile(ctx, specs, store.ReconcileOptions{})
	switch {
	case errors.Is(err, pgstore.ErrReconcileBusy):
		// A peer is already converging this schema to the same desired state;
		// its pass covers us. Not a failure — the specs are published above so
		// violations still map to a property.
		slog.Debug("appbuild: derived-schema reconcile skipped; a peer holds the lock")
		return
	case err != nil:
		slog.Warn("appbuild: derived-schema reconcile failed; uniqueness may be "+
			"enforced only by the application-level check until repaired", "error", err)
		return
	}
	for _, o := range outcomes {
		switch o.State {
		case store.DerivedUnenforced:
			slog.Warn("appbuild: derived unique constraint NOT enforced",
				"type", o.Spec.Type, "property", o.Spec.Property,
				"blocking_value_groups", o.BlockingCount, "reason", o.Reason)
		case store.DerivedCreated:
			slog.Info("appbuild: derived unique constraint created",
				"type", o.Spec.Type, "property", o.Spec.Property)
		case store.DerivedDropped:
			slog.Info("appbuild: derived schema object dropped (no longer declared)",
				"reason", o.Reason)
		case store.DerivedEnforced:
			// Already present and correct — the steady-state case, nothing to log.
		}
	}
}

// uniqueSpecsFromMetamodel collects the (type, property) pairs the derived-schema
// reconciler should enforce: every non-list property declared unique. Called at
// store-open with the boot-time metamodel; it is NOT re-invoked on a live schema
// reload (see Store.Reconcile's boot-only note).
func uniqueSpecsFromMetamodel(meta *metamodel.Metamodel) []store.DerivedObjectSpec {
	if meta == nil {
		return nil
	}
	var specs []store.DerivedObjectSpec
	for _, typeName := range meta.EntityTypes() {
		def, ok := meta.GetEntityDef(typeName)
		if !ok {
			continue
		}
		for propName, pd := range def.PropertyDefs() {
			if pd.Unique && !pd.List {
				specs = append(specs, store.DerivedObjectSpec{
					Kind:     store.DerivedUnique,
					Type:     typeName,
					Property: propName,
				})
			}
		}
	}
	return specs
}
