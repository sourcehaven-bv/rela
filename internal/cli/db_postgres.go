//go:build postgres

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// resolveDSN returns the database URL from RELA_DATABASE_URL, erroring if it is
// not set. The DSN is env-only (no flag) so the credential never appears in
// process listings or shell history.
func resolveDSN() (string, error) {
	dsn := os.Getenv("RELA_DATABASE_URL")
	if dsn == "" {
		return "", errors.New("no database URL: set RELA_DATABASE_URL")
	}
	return dsn, nil
}

// runDBMigrate applies pending migrations (postgres build). Pool construction
// lives in pgstore (MigrateDSN/StatusDSN) so the CLI doesn't depend on pgx.
func runDBMigrate() error {
	resolved, err := resolveDSN()
	if err != nil {
		return err
	}
	ctx := context.Background()
	before, target, err := pgstore.StatusDSN(ctx, resolved)
	if err != nil {
		return err
	}
	if before >= target {
		fmt.Printf("Database is up to date (schema version %d).\n", before)
		return nil
	}
	if err := pgstore.MigrateDSN(ctx, resolved); err != nil {
		return err
	}
	fmt.Printf("Applied migrations: schema version %d → %d.\n", before, target)
	return nil
}

// runDBStatus reports current vs target schema version. Exits non-zero when the
// database is behind, so CI can gate on it.
func runDBStatus() error {
	resolved, err := resolveDSN()
	if err != nil {
		return err
	}
	current, target, err := pgstore.StatusDSN(context.Background(), resolved)
	if err != nil {
		return err
	}
	if current < target {
		fmt.Printf("Database is BEHIND: schema version %d, binary expects %d.\n", current, target)
		fmt.Println("Run 'rela db migrate' to apply pending migrations.")
		os.Exit(1)
	}
	fmt.Printf("Database is up to date (schema version %d).\n", current)

	// Also report derived-schema drift (unique indexes vs the metamodel). This
	// is INFORMATIONAL: it never changes the exit code (a config edit against
	// dirty data must not brick a health check). The strict gate lives on
	// `rela db reconcile --dry-run`.
	if specs, _, ok := loadDerivedSpecs(); ok {
		outcomes, err := pgstore.ReconcileDSN(
			context.Background(), resolved, specs, store.ReconcileOptions{DryRun: true})
		switch {
		case errors.Is(err, pgstore.ErrReconcileBusy):
			fmt.Println("Derived schema: a reconcile is in progress; skipped.")
			return nil
		case err != nil:
			fmt.Printf("Derived schema: could not check (%v).\n", err)
			return nil
		}
		printDerivedDrift(outcomes, false)
	}
	return nil
}

// runDBReconcile converges (or, with dryRun, reports) the derived schema:
// partial unique indexes synthesized from the metamodel's `unique: true`
// properties. On --dry-run it exits non-zero if the live schema differs from
// the metamodel, so it can gate a deploy/CI step.
func runDBReconcile(dryRun, showValues bool) error {
	resolved, err := resolveDSN()
	if err != nil {
		return err
	}
	specs, schemaPath, ok := loadDerivedSpecs()
	if !ok {
		return errors.New("could not load project configuration to derive database indexes; " +
			"run this from a project directory")
	}
	// Echo which schema the constraints are derived from: reconcile mutates the
	// shared database toward THIS file, so an operator running from the wrong
	// checkout must be able to see it (RR-0USU3N).
	fmt.Printf("Reconciling derived schema from %s\n", schemaPath)
	outcomes, err := pgstore.ReconcileDSN(context.Background(), resolved, specs,
		store.ReconcileOptions{DryRun: dryRun, ShowValues: showValues})
	if errors.Is(err, pgstore.ErrReconcileBusy) {
		fmt.Println("Another reconcile is in progress for this schema; try again shortly.")
		return nil
	}
	if err != nil {
		return err
	}
	drift := printDerivedDrift(outcomes, dryRun)
	if dryRun && drift {
		os.Exit(1)
	}
	return nil
}

// printDerivedDrift prints the reconcile outcomes and reports whether any object
// drifted (was/would-be created, dropped, or is unenforced). When dryRun, the
// verbs are phrased as "would ...".
func printDerivedDrift(outcomes []store.DerivedObjectOutcome, dryRun bool) (drift bool) {
	var enforced int
	for _, o := range outcomes {
		switch o.State {
		case store.DerivedEnforced:
			enforced++
		case store.DerivedCreated:
			drift = true
			verb := "created"
			if dryRun {
				verb = "would create"
			}
			if o.Spec.Kind == store.DerivedQueryIndex {
				fmt.Printf("  + %s query index on %s.%v\n", verb, o.Spec.Type, o.Spec.Properties)
			} else {
				fmt.Printf("  + %s unique constraint on %s.%s\n", verb, o.Spec.Type, o.Spec.Property)
			}
		case store.DerivedDropped:
			drift = true
			verb := "dropped"
			if dryRun {
				verb = "would drop"
			}
			fmt.Printf("  - %s %s\n", verb, o.Reason)
		case store.DerivedUnenforced:
			drift = true
			if o.Spec.Kind == store.DerivedQueryIndex {
				fmt.Printf("  ! NOT created: query index on %s.%v — %s\n",
					o.Spec.Type, o.Spec.Properties, o.Reason)
			} else {
				fmt.Printf("  ! NOT enforced: unique on %s.%s — %s (%d duplicate value group(s))\n",
					o.Spec.Type, o.Spec.Property, o.Reason, o.BlockingCount)
			}
			for _, v := range o.SampleValues {
				fmt.Printf("      duplicate value: %s\n", v)
			}
		}
	}
	if !drift {
		fmt.Printf("Derived schema: up to date (%d object(s) enforced).\n", enforced)
	}
	return drift
}

// loadDerivedSpecs discovers the project and returns unique constraints plus
// indexes derived from valid static data-entry queries. ok is false when the
// complete desired set cannot be loaded; callers must not reconcile a partial
// set because that could drop still-desired owned indexes.
func loadDerivedSpecs() (specs []store.DerivedObjectSpec, schemaPath string, ok bool) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	startDir, err := os.Getwd()
	if err != nil {
		return nil, "", false
	}
	paths, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, "", false
	}
	meta, _, err := metamodel.NewFSLoader(fs, paths.SchemaPath).Load(context.Background())
	if err != nil {
		return nil, "", false
	}
	for _, typeName := range meta.EntityTypes() {
		def, defOK := meta.GetEntityDef(typeName)
		if !defOK {
			continue
		}
		for propName, pd := range def.PropertyDefs() {
			if pd.Unique && !pd.List {
				specs = append(specs, store.DerivedObjectSpec{
					Kind: store.DerivedUnique, Type: typeName, Property: propName,
				})
			}
		}
	}
	configPath := filepath.Join(paths.Root, dataentryconfig.ConfigFile)
	data, err := fs.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return specs, paths.SchemaPath, true
		}
		return nil, "", false
	}
	querySpecs, err := queryplan.LoadStaticIndexSpecs(data, meta)
	if err != nil {
		return nil, "", false
	}
	specs = append(specs, querySpecs...)
	return specs, paths.SchemaPath, true
}
