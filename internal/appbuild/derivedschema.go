//go:build postgres || sqlite

package appbuild

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// staticIndexSpecs derives the query and list index specs from the project's
// data-entry config. A missing config yields none. An unreadable or invalid one
// is an error, and the caller must then not reconcile at all: reconciliation
// drops every owned index absent from the desired set, so a partial set is
// destructive.
//
// It reads through the config seam rather than the filesystem. A packaged
// project carries data-entry.yaml in its database, and reading the file
// directly would find nothing there, silently dropping every derived index.
func staticIndexSpecs(
	ctx context.Context, meta *metamodel.Metamodel, cfg config.Loader,
) ([]store.DerivedObjectSpec, error) {
	data, err := cfg.Load(ctx, dataentryconfig.ConfigFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("data-entry config unreadable: %w", err)
	}
	specs, err := queryplan.LoadStaticIndexSpecs(data, meta)
	if err != nil {
		return nil, fmt.Errorf("invalid data-entry config: %w", err)
	}
	return specs, nil
}

// logDerivedOutcomes reports what a boot-time reconcile changed. The steady
// state, every object already enforced, logs nothing.
func logDerivedOutcomes(outcomes []store.DerivedObjectOutcome) {
	for _, o := range outcomes {
		switch o.State {
		case store.DerivedUnenforced:
			if o.Spec.Kind == store.DerivedUnique {
				slog.Warn("appbuild: derived unique constraint NOT enforced",
					"type", o.Spec.Type, "property", o.Spec.Property,
					"blocking_value_groups", o.BlockingCount, "reason", o.Reason)
			} else {
				slog.Warn("appbuild: derived index NOT created", "kind", o.Spec.Kind,
					"type", o.Spec.Type, "properties", o.Spec.Properties, "order_by", o.Spec.OrderBy,
					"reason", o.Reason)
			}
		case store.DerivedCreated:
			if o.Spec.Kind == store.DerivedUnique {
				slog.Info("appbuild: derived unique constraint created",
					"type", o.Spec.Type, "property", o.Spec.Property)
			} else {
				slog.Info("appbuild: derived index created", "kind", o.Spec.Kind,
					"type", o.Spec.Type, "properties", o.Spec.Properties, "order_by", o.Spec.OrderBy)
			}
		case store.DerivedDropped:
			slog.Info("appbuild: derived schema object dropped (no longer declared)", "reason", o.Reason)
		case store.DerivedEnforced:
		}
	}
}
