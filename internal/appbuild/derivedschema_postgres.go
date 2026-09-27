//go:build postgres

package appbuild

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/config"
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

// reconcileDerivedSchemaIfSupported converges unique constraints and eligible
// static-query indexes at store-open. It also publishes unique pairs so the
// write path can attribute a unique-index violation to a property. A store
// without the capability is skipped. Reconcile failures are logged and
// swallowed: a derived-schema problem must never fail store-open. An operator
// inspects or repairs drift via `rela db status` / `rela db reconcile`.
func reconcileDerivedSchemaIfSupported(
	ctx context.Context, st store.Store, base *SharedBase, cfg config.Loader,
) {
	s, ok := st.(derivedSchemaReconciler)
	if !ok {
		return
	}

	uniqueSpecs := uniqueSpecsFromMetamodel(base.meta)
	// Publish unique pairs even when data-entry config later prevents DDL. An
	// already-present unique index may still reject a concurrent write, and the
	// error must remain attributable to its property.
	s.SetUniqueSpecProvider(uniqueSpecs)
	querySpecs, err := staticIndexSpecs(ctx, base.meta, cfg)
	if err != nil {
		slog.Warn("appbuild: derived-schema reconcile skipped", "error", err)
		return
	}
	specs := append(append([]store.DerivedObjectSpec(nil), uniqueSpecs...), querySpecs...)

	outcomes, err := s.Reconcile(ctx, specs, store.ReconcileOptions{})
	switch {
	case errors.Is(err, pgstore.ErrReconcileBusy):
		// A peer is already converging this schema to the same desired state;
		// its pass covers us. Not a failure — the specs are published above so
		// violations still map to a property.
		slog.Debug("appbuild: derived-schema reconcile skipped; a peer holds the lock")
		return
	case err != nil:
		slog.Warn("appbuild: derived-schema reconcile failed; database constraints or query indexes may be stale",
			"error", err)
		return
	}
	logDerivedOutcomes(outcomes)
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
