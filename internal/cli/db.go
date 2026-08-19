package cli

// DBCmd groups database-administration subcommands for the PostgreSQL build.
// The schema is applied automatically when the store first opens (see
// pgstore.Open); these commands exist for operators who want to apply or check
// migrations explicitly — e.g. as a separate, privileged deploy step, or a CI
// gate — rather than relying on auto-migrate.
//
// The subcommands are only functional in the `postgres` build. In the default
// (filesystem) and `memorybackend` builds they return a clear "not available"
// error (see runDBMigrate / runDBStatus in the build-tagged db_*.go files).
type DBCmd struct {
	Migrate   DBMigrateCmd   `cmd:"" help:"Apply pending PostgreSQL schema migrations."`
	Status    DBStatusCmd    `cmd:"" help:"Report the database schema version (read-only; non-zero exit if behind)."`
	Reconcile DBReconcileCmd `cmd:"" help:"Converge derived-schema objects (unique indexes) with the metamodel."`
}

// DBMigrateCmd applies pending schema migrations to the database named by the
// RELA_DATABASE_URL environment variable. Idempotent: a no-op when already
// current. The DSN is env-only (no flag) so the credential never appears on a
// command line.
type DBMigrateCmd struct{}

// Run executes `rela db migrate`.
func (c *DBMigrateCmd) Run() error {
	return runDBMigrate()
}

// DBStatusCmd reports the current vs target schema version without changing
// anything. Exits non-zero when the database is behind (for CI gating). Reads
// the DSN from RELA_DATABASE_URL (env-only).
type DBStatusCmd struct{}

// Run executes `rela db status`.
func (c *DBStatusCmd) Run() error {
	return runDBStatus()
}

// DBReconcileCmd converges the database's derived-schema objects — partial
// unique indexes synthesized from the metamodel's `unique: true` properties
// (TKT-3Q0GP1) — creating missing ones and dropping ones no longer declared.
//
// This is the explicit operator affordance for the same reconciliation that
// runs automatically at store-open. Its trust boundary is the operator shell
// (like `db migrate`): it takes no ACL and reads the DSN from RELA_DATABASE_URL.
//
// With --dry-run it computes and prints the plan WITHOUT changing anything, and
// exits non-zero if the live schema differs from the metamodel — a pre-flight/CI
// gate an operator can run before deploying a schema change. Blocking duplicate
// values are reported as a COUNT by default; --show-values additionally prints a
// bounded sample of the offending values (entity content, so opt-in only).
type DBReconcileCmd struct {
	DryRun     bool `help:"Print the plan and exit non-zero on drift, without changing the database."`
	ShowValues bool `help:"Include sample blocking values for unenforced constraints (entity content; operator use only)."`
}

// Run executes `rela db reconcile`.
func (c *DBReconcileCmd) Run() error {
	return runDBReconcile(c.DryRun, c.ShowValues)
}
