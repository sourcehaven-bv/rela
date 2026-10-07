package cli

import "context"

// DBCmd groups database-administration subcommands for the PostgreSQL and
// SQLite builds. The schema is applied automatically when the store first
// opens; these commands exist for operators who want to apply or check
// migrations explicitly — e.g. as a separate, privileged deploy step, or a CI
// gate — rather than relying on auto-migrate.
//
// The subcommands are only functional in the `postgres` and `sqlite` builds.
// In the default (filesystem) and `memorybackend` builds they return a clear "not available"
// error (see runDBMigrate / runDBStatus in the build-tagged db_*.go files).
type DBCmd struct {
	Migrate   DBMigrateCmd   `cmd:"" help:"Apply pending PostgreSQL schema migrations."`
	Status    DBStatusCmd    `cmd:"" help:"Report the database schema version (read-only; non-zero exit if behind)."`
	Reconcile DBReconcileCmd `cmd:"" help:"Converge derived-schema objects (unique and query indexes) with the configuration."`
	Load      DBLoadCmd      `cmd:"" help:"Store the project's config files (schema, data-entry, ACL, scripts, templates) and, with --data, its markdown data in the database (SQLite)."`
	Dump      DBDumpCmd      `cmd:"" help:"Write the config files and, with --data, the data stored in the database to a directory (SQLite)."`
	ImportFS  DBImportFSCmd  `cmd:"" name:"import-fs" help:"Copy a filesystem project into a new SQLite project directory (SQLite build only)."`
}

// DBLoadCmd bakes a project's operator-authored config into its SQLite
// database, so the database file alone is a working project (FEAT-UP14BT).
//
// The stored set is REPLACED, not merged: a file removed from the source
// directory is removed from the database too. Files on disk still take
// precedence over the stored copy when both exist, so a project being
// edited keeps reading what the operator just wrote.
//
// With --data it also copies the markdown project's entities, relations
// and attachments into the database. That copy is a raw-store write under
// the same terms as `rela dev seed`: attributed to the "fs-import" tool,
// one audit record per run, and refused when the database already holds
// entities unless --force is given. It runs in one transaction, so a
// failed import writes nothing. The data is imported before the config is
// stored, so a refused import leaves the database untouched.
//
// Like `db migrate`, the trust boundary is the operator shell; it takes no
// ACL. It opens the database, so it fails while a server has it open.
type DBLoadCmd struct {
	From  string `help:"Directory to read config and data from (default: the project root)." type:"existingdir"`
	Data  bool   `help:"Also import the markdown entities, relations and attachments (raw store write, no automations or validation)."`
	Force bool   `help:"With --data, import even though the database already holds entities. An id that is already stored still fails the import."`
}

// Run executes `rela db load`.
func (c *DBLoadCmd) Run(ctx context.Context) error {
	return runDBLoad(ctx, c.From, c.Data, c.Force)
}

// DBDumpCmd writes the config stored in the SQLite database out as files,
// the export half of `db load`: dump, edit, load.
//
// With --data it also writes the entities, relations and attachments as
// markdown, so the directory becomes a project the filesystem build opens.
// That part refuses a directory that already has entities/, relations/ or
// attachments/, with or without --force.
type DBDumpCmd struct {
	Dir   string `arg:"" help:"Directory to write the config files into."`
	Data  bool   `help:"Also write the entities, relations and attachments as markdown."`
	Force bool   `help:"Overwrite config files that already exist. Existing data directories are never written into."`
}

// Run executes `rela db dump`.
func (c *DBDumpCmd) Run() error {
	return runDBDump(c.Dir, c.Data, c.Force)
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

// DBReconcileCmd converges the database's derived-schema objects, creating
// missing ones and dropping ones no longer declared: partial unique indexes
// from the metamodel's `unique: true` properties (TKT-3Q0GP1, postgres only)
// and query and list indexes from data-entry.yaml (both database builds).
//
// This is the explicit operator affordance for the same reconciliation that
// runs automatically at store-open. Its trust boundary is the operator shell
// (like `db migrate`): it takes no ACL. On postgres it reads the DSN from
// RELA_DATABASE_URL.
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

// DBImportFSCmd copies a filesystem project into a new project directory
// whose data lives in a SQLite database (TKT-YNKKRQ). The source is left
// unchanged and the target must not exist yet. Only the SQLite build
// implements it; see internal/fsimport for what is copied.
//
// Like the other db commands it takes no project from the working directory:
// both directories are named on the command line, so running it inside the
// source can never open a database there.
type DBImportFSCmd struct {
	Source string `arg:"" help:"The filesystem project to copy." type:"path"`
	Target string `arg:"" help:"The directory to create for the SQLite project. Must not exist." type:"path"`
}

// Run executes `rela db import-fs`.
func (c *DBImportFSCmd) Run(ctx context.Context) error {
	return runDBImportFS(ctx, c.Source, c.Target)
}
