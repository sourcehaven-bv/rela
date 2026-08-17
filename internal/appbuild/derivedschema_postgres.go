//go:build postgres

package appbuild

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// reconcileDerivedSchemaIfSupported converges the postgres derived schema at
// store-open and publishes the metamodel's unique (type, property) pairs so the
// write path can attribute a unique-index violation to a property (TKT-3Q0GP1).
// A non-pgstore store (should not happen in this build) is skipped. Reconcile
// failures are logged and swallowed: a derived-schema problem — most often
// pre-existing duplicate values blocking an index — must NEVER fail store-open.
// An operator inspects/repairs drift via `rela db status` / `rela db reconcile`.
func reconcileDerivedSchemaIfSupported(ctx context.Context, st store.Store, meta *metamodel.Metamodel) {
	s, ok := st.(*pgstore.Store)
	if !ok {
		return
	}

	specs := uniqueSpecsFromMetamodel(meta)

	// Publish the pairs first so that even if the reconcile below degrades, a
	// violation of an already-present index still maps to its property.
	s.SetUniqueSpecProvider(specs)

	outcomes, err := s.Reconcile(ctx, specs, store.ReconcileOptions{})
	if err != nil {
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
// reconciler should enforce: every non-list property declared unique. Read fresh
// from the metamodel so a reload is reflected on the next call.
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
