//go:build postgres

package appbuild

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
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
func reconcileDerivedSchemaIfSupported(ctx context.Context, st store.Store, base *SharedBase) {
	s, ok := st.(derivedSchemaReconciler)
	if !ok {
		return
	}

	uniqueSpecs := uniqueSpecsFromMetamodel(base.meta)
	// Publish unique pairs even when data-entry config later prevents DDL. An
	// already-present unique index may still reject a concurrent write, and the
	// error must remain attributable to its property.
	s.SetUniqueSpecProvider(uniqueSpecs)
	specs := append([]store.DerivedObjectSpec(nil), uniqueSpecs...)
	configPath := filepath.Join(base.cfg.Paths.Root, dataentryconfig.ConfigFile)
	if data, err := base.cfg.FS.ReadFile(configPath); err == nil {
		querySpecs, err := queryplan.LoadStaticIndexSpecs(data, base.meta)
		if err != nil {
			slog.Warn("appbuild: derived-schema reconcile skipped; invalid data-entry config", "error", err)
			return
		}
		specs = append(specs, querySpecs...)
	} else if !os.IsNotExist(err) {
		slog.Warn("appbuild: derived-schema reconcile skipped; data-entry config unreadable", "error", err)
		return
	}

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
	for _, o := range outcomes {
		switch o.State {
		case store.DerivedUnenforced:
			if o.Spec.Kind == store.DerivedQueryIndex {
				slog.Warn("appbuild: derived static-query index NOT created",
					"type", o.Spec.Type, "properties", o.Spec.Properties, "reason", o.Reason)
			} else {
				slog.Warn("appbuild: derived unique constraint NOT enforced",
					"type", o.Spec.Type, "property", o.Spec.Property,
					"blocking_value_groups", o.BlockingCount, "reason", o.Reason)
			}
		case store.DerivedCreated:
			if o.Spec.Kind == store.DerivedQueryIndex {
				slog.Info("appbuild: derived static-query index created",
					"type", o.Spec.Type, "properties", o.Spec.Properties)
			} else {
				slog.Info("appbuild: derived unique constraint created",
					"type", o.Spec.Type, "property", o.Spec.Property)
			}
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
