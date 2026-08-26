package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// The `rela migrate status|gen|data|gc` subcommands operate the
// data-migration system (TKT-0C57FS). Trust boundary is the operator shell,
// like `db migrate` and `history-purge`: raw store access, no ACL, explicit
// audit. All four bind *writeServices (matched in requiresProject by full
// command path — bare `rela migrate` stays service-free).

// loadDataMigrations parses the project's migrations/ directory.
func loadDataMigrations(svc *writeServices) ([]*datamigration.File, error) {
	return datamigration.LoadDir(os.DirFS(svc.Paths.Root))
}

// migrationLock builds the per-store lock the same way appbuild does, so
// CLI runs and server-side gate/GC writers exclude each other. Each command
// invocation builds ONE lock value and threads it everywhere — for the fs
// implementation the embedded in-process mutex only spans one value.
func migrationLock(svc *writeServices) datamigration.MigrationLock {
	return datamigration.LockFor(svc.Store, svc.Paths.CacheDir)
}

// currentShape loads the marker (bootstrapping it via the gate when absent)
// and returns the shape the data conforms to plus the gate for reuse.
func evaluateGate(
	ctx context.Context, svc *writeServices, lock datamigration.MigrationLock,
) (*datamigration.Gate, *datamigration.Verdict, error) {
	gate, err := datamigration.NewGate(svc.State, lock)
	if err != nil {
		return nil, nil, err
	}
	v, err := gate.Evaluate(ctx, svc.Meta)
	if err != nil {
		return nil, nil, err
	}
	return gate, v, nil
}

// MigrateStatusCmd shows where the store's data stands relative to the live
// schema shape.
type MigrateStatusCmd struct{}

// Run executes `rela migrate status`.
func (c *MigrateStatusCmd) Run(ctx context.Context, svc *writeServices) error {
	_, v, err := evaluateGate(ctx, svc, migrationLock(svc))
	if err != nil {
		return err
	}
	fmt.Println(v.Describe())
	for _, tier := range []metamodel.ShapeTier{metamodel.TierMigration, metamodel.TierDrift} {
		for _, d := range v.Report.ByTier(tier) {
			fmt.Printf("  [%s] %s\n", tier, d.Detail)
		}
	}
	files, err := loadDataMigrations(svc)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		fmt.Printf("%d migration file(s) in %s/\n", len(files), datamigration.MigrationsDir)
	}
	if v.Status == datamigration.StatusNeedsMigration {
		marker, mErr := datamigration.LoadMarker(ctx, svc.State)
		if mErr != nil {
			return mErr
		}
		stored, pErr := marker.ShapeProjection()
		if pErr != nil {
			return pErr
		}
		if _, rErr := datamigration.Resolve(stored, marker.Applied, svc.Meta.ShapeProjection(), files); rErr != nil {
			fmt.Println("no resolvable migration path yet — run `rela migrate gen`")
		} else {
			fmt.Println("a migration path exists — run `rela migrate data` to preview it")
		}
		return relaerrors.NewExitError(1)
	}
	return nil
}

// MigrateGenCmd drafts a migration file from the shape diff between the
// marker and the live schema.
type MigrateGenCmd struct {
	Description string `help:"One-line description embedded in the migration file." default:""`
	Stdout      bool   `help:"Print the draft instead of writing migrations/<name>."`
}

// Run executes `rela migrate gen`.
func (c *MigrateGenCmd) Run(ctx context.Context, svc *writeServices) error {
	marker, err := datamigration.LoadMarker(ctx, svc.State)
	if err != nil {
		return err
	}
	if marker == nil {
		// No marker: nothing recorded to diff against. Evaluate the gate
		// (bootstraps the baseline) and tell the operator why gen is a no-op.
		if _, _, gErr := evaluateGate(ctx, svc, migrationLock(svc)); gErr != nil {
			return gErr
		}
		fmt.Println("no recorded data shape yet — baseline adopted; edit schema.yaml first, then re-run `rela migrate gen`")
		return nil
	}
	stored, err := marker.ShapeProjection()
	if err != nil {
		return err
	}
	existing, err := loadDataMigrations(svc)
	if err != nil {
		return err
	}
	draft, err := datamigration.Generate(stored, svc.Meta.ShapeProjection(), existing, c.Description)
	if err != nil {
		return err
	}
	if draft == nil {
		fmt.Println("no shape change needing a migration — nothing generated")
		return nil
	}
	if c.Stdout {
		fmt.Print(string(draft.Content))
		return nil
	}
	dir := filepath.Join(svc.Paths.Root, datamigration.MigrationsDir)
	if err := svc.FS.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(dir, draft.FileName)
	if _, err := svc.FS.Stat(target); err == nil {
		return fmt.Errorf("refusing to overwrite existing %s", target)
	}
	if err := svc.FS.WriteFile(target, draft.Content, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s — REVIEW the GUESS/TODO annotations, then run `rela migrate data`\n",
		filepath.Join(datamigration.MigrationsDir, draft.FileName))
	return nil
}

// MigrateDataCmd resolves and runs the pending migration chain. Dry-run by
// default; --apply executes.
type MigrateDataCmd struct {
	Apply bool `help:"Apply the migrations (default is a dry-run preview)."`
}

// Run executes `rela migrate data [--apply]`.
func (c *MigrateDataCmd) Run(ctx context.Context, svc *writeServices) error {
	lock := migrationLock(svc)
	marker, err := datamigration.LoadMarker(ctx, svc.State)
	if err != nil {
		return err
	}
	if marker == nil {
		if _, _, gErr := evaluateGate(ctx, svc, lock); gErr != nil {
			return gErr
		}
		fmt.Println("no recorded data shape yet — baseline adopted; nothing to migrate")
		return nil
	}
	stored, err := marker.ShapeProjection()
	if err != nil {
		return err
	}
	files, err := loadDataMigrations(svc)
	if err != nil {
		return err
	}
	live := svc.Meta.ShapeProjection()
	plan, err := datamigration.Resolve(stored, marker.Applied, live, files)
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		// Nothing to run; let the gate adopt any compatible remainder.
		_, v, gErr := evaluateGate(ctx, svc, lock)
		if gErr != nil {
			return gErr
		}
		fmt.Println(v.Describe())
		return nil
	}

	runner, err := datamigration.NewRunner(datamigration.Deps{
		Store:    svc.Store,
		Meta:     svc.Meta,
		State:    svc.State,
		Audit:    svc.Audit,
		ScriptFS: os.DirFS(svc.Paths.Root),
		Versions: versionCaptureFor(svc),
		Lock:     lock,
	})
	if err != nil {
		return err
	}
	res, err := runner.Run(ctx, plan, c.Apply)
	printRunResult(res, c.Apply)
	if err != nil {
		return err
	}
	if c.Apply {
		// Let the gate adopt any compatible tail gap and publish in-sync.
		if _, v, gerr := evaluateGate(ctx, svc, lock); gerr == nil && v != nil {
			fmt.Println(v.Describe())
		}
	} else {
		fmt.Println("dry-run only — re-run with --apply to execute")
	}
	return nil
}

func printRunResult(res *datamigration.RunResult, applied bool) {
	if res == nil {
		return
	}
	verb := "would change"
	if applied {
		verb = "changed"
	}
	for _, f := range res.Files {
		fmt.Printf("%s (%.12s → %.12s):\n", f.Name, f.From, f.To)
		for _, s := range f.Steps {
			fmt.Printf("  %-22s %-40s %s %d record(s)\n", s.Kind, s.Target, verb, s.Affected)
			for _, n := range s.Notes {
				fmt.Printf("    note: %s\n", n)
			}
		}
	}
	fmt.Printf("entities failing live-schema validation: %d before, %d after\n",
		res.ValidationBefore, res.ValidationAfter)
}

// MigrateGCCmd runs the schema-orphan garbage collector once. Dry-run by
// default; --apply deletes expired drift; --scan reconciles the ledger from
// a full content read first (legacy orphans that never crossed the gate).
type MigrateGCCmd struct {
	Apply bool          `help:"Delete expired orphaned data (default is a dry-run preview)."`
	Scan  bool          `help:"Full-scan the store for orphans not yet in the drift ledger."`
	Grace time.Duration `help:"Override the grace period orphaned data must age before deletion (default 720h)." default:"0"`
}

// Run executes `rela migrate gc [--scan] [--apply]`.
func (c *MigrateGCCmd) Run(ctx context.Context, svc *writeServices) error {
	lock := migrationLock(svc)
	gate, _, err := evaluateGate(ctx, svc, lock)
	if err != nil {
		return err
	}
	gc, err := datamigration.NewGC(datamigration.GCDeps{
		Store:    svc.Store,
		Meta:     func() *metamodel.Metamodel { return svc.Meta },
		State:    svc.State,
		Audit:    svc.Audit,
		Verdicts: gate,
		Versions: versionCaptureFor(svc),
		Grace:    c.Grace,
		Lock:     lock,
	})
	if err != nil {
		return err
	}
	if c.Scan {
		added, sErr := gc.Scan(ctx)
		switch {
		case errors.Is(sErr, datamigration.ErrLockHeld):
			// A concurrent run (e.g. the server's hourly sweep) holds the
			// lock; the scan is additive bookkeeping, so report and carry on
			// to the tick preview rather than failing the whole command.
			fmt.Println("scan skipped: another migration or GC run is active — re-run later")
		case sErr != nil:
			return sErr
		default:
			fmt.Printf("scan added %d orphan(s) to the drift ledger\n", len(added))
			for _, key := range added {
				fmt.Printf("  %s\n", key)
			}
		}
	}
	res, err := gc.Tick(ctx, c.Apply)
	if err != nil {
		return err
	}
	if res.Skipped != "" {
		fmt.Printf("gc skipped: %s\n", res.Skipped)
		return nil
	}
	verb := "would delete"
	if c.Apply {
		verb = "deleted"
	}
	for _, d := range res.Deleted {
		fmt.Printf("%s %s (%s): %d record(s)\n", verb, d.Key, d.Kind, d.Affected)
	}
	for _, p := range res.Pending {
		fmt.Printf("pending %s (%s): grace expires %s\n", p.Key, p.Kind, p.Deadline.Format("2006-01-02"))
	}
	if !c.Apply && len(res.Deleted) > 0 {
		fmt.Println("dry-run only — re-run with --apply to delete")
	}
	return nil
}

// versionCaptureFor returns the synchronous version-capture sink when the
// backend has one (pgstore), nil otherwise — the runner and GC degrade to
// no capture, where git is the recovery path.
func versionCaptureFor(svc *writeServices) datamigration.VersionCapture {
	if svc.Versions == nil {
		return nil
	}
	return svc.Versions
}
