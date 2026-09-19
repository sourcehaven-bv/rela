package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// The `rela migrate status|gen|data|gc` subcommands operate the
// data-migration system (TKT-0C57FS). Trust boundary is the operator shell,
// like `db migrate` and `history-purge`: raw store access, no ACL, explicit
// audit. All four bind *writeServices (matched in requiresProject by full
// command path — bare `rela migrate` stays service-free).

// loadDataMigrations parses the project's migrations/ directory.
//
// Reads through the injected [config.Loader] rather than os.DirFS so a project
// whose config lives somewhere other than the filesystem — a self-contained
// SQLite file — still has a migration chain.
func loadDataMigrations(ctx context.Context, svc *writeServices) ([]*datamigration.File, error) {
	fsys, err := newConfigFS(ctx, svc)
	if err != nil {
		return nil, err
	}
	return datamigration.LoadDir(fsys)
}

// newConfigFS views the project's config as an fs.FS, bound to ctx.
//
// The error is a wiring failure (a nil Config), not a runtime condition, but
// it is returned rather than panicked on for the reason CLAUDE.md gives: the
// call sites already thread errors, and a startup error names the problem
// where a panic only names the symptom.
func newConfigFS(ctx context.Context, svc *writeServices) (fs.FS, error) {
	v, err := config.NewFSView(svc.Config)
	if err != nil {
		return nil, err
	}
	return v.WithContext(ctx)
}

// migrationLock builds the per-store lock the same way appbuild does, so
// CLI runs and server-side gate/GC writers exclude each other. Each command
// invocation builds ONE lock value and threads it everywhere — for the fs
// implementation the embedded in-process mutex only spans one value.
func migrationLock(svc *writeServices) datamigration.MigrationLock {
	return datamigration.LockFor(svc.Store, svc.Paths.CacheDir)
}

// newGate builds the gate over this store's migration record and drift ledger.
func newGate(svc *writeServices, lock datamigration.MigrationLock) (*datamigration.Gate, error) {
	return datamigration.NewGate(datamigration.GateDeps{
		MigState: svc.MigState,
		State:    svc.State,
		Lock:     lock,
		// The shared name-only probe, so the CLI and appbuild cannot answer
		// the un-baselined question differently. A full parse here would let
		// one malformed file turn "does this project have migrations?" into an
		// error, which the gate would surface as a refusal to classify.
		HasMigrations: func(ctx context.Context) (bool, error) {
			fsys, err := newConfigFS(ctx, svc)
			if err != nil {
				return false, err
			}
			return datamigration.HasMigrations(fsys)
		},
	})
}

// evaluateGate classifies without writing, for read-only commands.
func evaluateGate(
	ctx context.Context, svc *writeServices, lock datamigration.MigrationLock,
) (*datamigration.Gate, *datamigration.Verdict, error) {
	gate, err := newGate(svc, lock)
	if err != nil {
		return nil, nil, err
	}
	v, err := gate.Evaluate(ctx, svc.Meta)
	if err != nil {
		return nil, nil, err
	}
	return gate, v, nil
}

// evaluateAndPersistGate classifies AND records a compatible adoption.
//
// This is the write half of the split on [datamigration.Gate]: the CLI is the
// only caller that persists, because on the filesystem tier the record is a
// git-tracked file and a server must not write one at boot.
func evaluateAndPersistGate(
	ctx context.Context, svc *writeServices, lock datamigration.MigrationLock,
) (*datamigration.Verdict, error) {
	gate, err := newGate(svc, lock)
	if err != nil {
		return nil, err
	}
	return gate.EvaluateAndPersist(ctx, svc.Meta)
}

// storedShape returns the shape the store's content conforms to, or false when
// nothing is recorded yet.
func storedShape(
	ctx context.Context, svc *writeServices,
) (proj metamodel.ShapeProjection, applied []string, recorded bool, err error) {
	st, loadErr := svc.MigState.Load(ctx)
	if loadErr != nil || st == nil {
		return metamodel.ShapeProjection{}, nil, false, loadErr
	}
	proj, projErr := st.ShapeProjection()
	if projErr != nil {
		return metamodel.ShapeProjection{}, nil, false, projErr
	}
	return proj, st.AppliedNames(), true, nil
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
	files, err := loadDataMigrations(ctx, svc)
	if err != nil {
		return err
	}
	if len(files) > 0 {
		fmt.Printf("%d migration file(s) in %s/\n", len(files), datamigration.MigrationsDir)
	}
	if v.Status == datamigration.StatusNeedsMigration {
		stored, applied, ok, sErr := storedShape(ctx, svc)
		if sErr != nil {
			return sErr
		}
		if ok {
			if _, rErr := datamigration.Resolve(stored, applied, svc.Meta.ShapeProjection(), files); rErr != nil {
				fmt.Println("the pending migrations do not cover this change — run `rela migrate gen`")
			} else {
				fmt.Println("pending migrations cover it — run `rela migrate data` to preview them")
			}
		}
		return relaerrors.NewExitError(1)
	}
	if v.Status == datamigration.StatusUnbaselined {
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
	stored, _, ok, err := storedShape(ctx, svc)
	if err != nil {
		return err
	}
	if !ok {
		// Nothing recorded to diff against. Evaluate and persist (which
		// baselines a project with no migrations) and say why gen is a no-op.
		v, gErr := evaluateAndPersistGate(ctx, svc, migrationLock(svc))
		if gErr != nil {
			return gErr
		}
		if v.Status == datamigration.StatusUnbaselined {
			fmt.Println(v.Describe())
			return relaerrors.NewExitError(1)
		}
		fmt.Println("no recorded data shape yet — baseline adopted; edit schema.yaml first, then re-run `rela migrate gen`")
		return nil
	}
	draft, err := datamigration.Generate(stored, svc.Meta.ShapeProjection(), c.Description, time.Now())
	if err != nil {
		return err
	}
	if draft == nil {
		fmt.Println("no schema change needing a migration — nothing generated " +
			"(a data-only migration is hand-written: create migrations/<timestamp>-<name>.yaml)")
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
	files, err := loadDataMigrations(ctx, svc)
	if err != nil {
		return err
	}
	stored, applied, ok, err := storedShape(ctx, svc)
	if err != nil {
		return err
	}
	if !ok {
		// No record yet. With no migrations on disk this is a new store and
		// the gate baselines it. With migrations present it is the
		// un-baselined case, and `migrate data` is exactly where the operator
		// resolves it: run every file from the beginning, because nothing says
		// any of them has run.
		if len(files) == 0 {
			if _, gErr := evaluateAndPersistGate(ctx, svc, lock); gErr != nil {
				return gErr
			}
			fmt.Println("no recorded data shape yet — baseline adopted; nothing to migrate")
			return nil
		}
		// Migrations exist and nothing is recorded, so what shape this store's
		// content conforms to is genuinely unknown. REFUSE rather than guess.
		//
		// The obvious guess — adopt files[0].FromProjection and replan the
		// whole chain — is what this command must not do. A store already at
		// the chain's end (its record lost, or gitignored by accident) would
		// have every migration re-run against migrated content, and the
		// residual compatibility check cannot catch it: with every file
		// planned, the walk ends at the last file's to-shape, which equals
		// live by construction. The one diagnostic that would notice is
		// structurally blind on exactly this path.
		//
		// This is the case StatusUnbaselined exists for, so defer to it. The
		// operator resolves it explicitly with `rela migrate baseline` (the
		// content already matches) or by re-running from a known state.
		fmt.Printf("%d migration file(s) in %s/, but this store has no recorded migration state.\n",
			len(files), datamigration.MigrationsDir)
		fmt.Println("Refusing to guess which have already run — running them all could re-apply " +
			"migrations against already-migrated content.")
		fmt.Println("If the content already matches the current schema, record it with " +
			"`rela migrate baseline --apply`.")
		return relaerrors.NewExitError(1)
	}
	live := svc.Meta.ShapeProjection()
	plan, err := datamigration.Resolve(stored, applied, live, files)
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

	// Same seam as loadDataMigrations: a `lua:` step's script is
	// operator-authored config, so it resolves wherever the rest of the
	// project's config does.
	//
	// Bound to ctx, which is the point of doing it here rather than reusing a
	// background view: the runner already binds a `lua:` step's VM to this
	// context so a runaway migration is interruptible, and reading the script
	// through an uncancellable FS would defeat half of that.
	scriptFS, err := newConfigFS(ctx, svc)
	if err != nil {
		return err
	}

	runner, err := datamigration.NewRunner(datamigration.Deps{
		Store:    svc.Store,
		Meta:     svc.Meta,
		State:    svc.State,
		MigState: svc.MigState,
		Audit:    svc.Audit,
		ScriptFS: scriptFS,
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
		fmt.Printf("%s:\n", f.Name)
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

// MigrateBaselineCmd records the project's migrations as already applied
// without running them, and adopts the live schema shape as the baseline.
//
// It is the escape hatch for the un-baselined case: a store with content that
// already matches the current schema, but no recorded migration state — an
// existing project adopting the migration system, or one whose state file was
// lost. Flyway's `baselineOnMigrate` fills the same role.
//
// It is deliberately a separate, explicit command rather than something the
// gate infers. Marking migrations applied when they are NOT is how data
// silently fails to be transformed, so the claim has to be the operator's.
type MigrateBaselineCmd struct {
	Apply bool `help:"Record the baseline (default is a dry-run preview)."`
}

// Run executes `rela migrate baseline [--apply]`.
func (c *MigrateBaselineCmd) Run(ctx context.Context, svc *writeServices) error {
	files, err := loadDataMigrations(ctx, svc)
	if err != nil {
		return err
	}

	// The load-decide-save below must be serialized against every other writer
	// of this record, exactly as the runner and the gate are (TKT-CPCBR7).
	// Without it, a `migrate data --apply` that starts first and records its
	// first file is clobbered by a baseline whose `existing == nil` check ran
	// before that write landed — and the migrations baseline then claims are
	// applied never run, which is the failure `StatusUnbaselined` exists to
	// prevent.
	//
	// Unlike the gate, contention FAILS rather than skipping: a baseline is an
	// explicit operator claim, and silently doing nothing would leave the
	// operator believing a claim that was never recorded.
	release, err := migrationLock(svc).TryAcquire(ctx)
	if err != nil {
		return err
	}
	defer release()

	existing, err := svc.MigState.Load(ctx)
	if err != nil {
		return err
	}
	if existing != nil {
		fmt.Println("migration state is already recorded — nothing to baseline")
		fmt.Println("(to re-run a migration, remove its entry from the applied list)")
		return nil
	}

	live := svc.Meta.ShapeProjection()
	entries := make([]datamigration.AppliedEntry, 0, len(files))
	now := time.Now().UTC()
	for _, f := range files {
		entries = append(entries, datamigration.AppliedEntry{Name: f.Name, AppliedAt: now})
	}

	if !c.Apply {
		fmt.Printf("would record %d migration(s) as already applied:\n", len(entries))
		for _, e := range entries {
			fmt.Printf("  %s\n", e.Name)
		}
		fmt.Println("dry-run only — re-run with --apply to record")
		return nil
	}

	st, err := datamigration.NewState(live, entries, now)
	if err != nil {
		return err
	}
	if err := svc.MigState.Save(ctx, st); err != nil {
		return err
	}
	svc.Audit.Record(audit.Record{
		Time:      now,
		Op:        audit.OpDataMigration,
		Principal: principal.From(ctx),
		Summary: fmt.Sprintf("migration baseline recorded: %d migration(s) marked applied without running",
			len(entries)),
	})
	fmt.Printf("recorded %d migration(s) as already applied\n", len(entries))
	return nil
}
