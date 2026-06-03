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
	Migrate DBMigrateCmd `cmd:"" help:"Apply pending PostgreSQL schema migrations."`
	Status  DBStatusCmd  `cmd:"" help:"Report the database schema version (read-only; non-zero exit if behind)."`
}

// DBMigrateCmd applies pending schema migrations to the database named by
// --database-url / RELA_DATABASE_URL. Idempotent: a no-op when already current.
type DBMigrateCmd struct{}

// Run executes `rela db migrate`.
func (c *DBMigrateCmd) Run() error {
	return runDBMigrate(databaseURL)
}

// DBStatusCmd reports the current vs target schema version without changing
// anything. Exits non-zero when the database is behind (for CI gating).
type DBStatusCmd struct{}

// Run executes `rela db status`.
func (c *DBStatusCmd) Run() error {
	return runDBStatus(databaseURL)
}
