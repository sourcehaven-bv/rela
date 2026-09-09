---
id: RR-IB66ZI
type: review-response
title: runDBMigrate re-read the version after migrating, and databasePath skipped SafeFS
finding: Two smaller issues in db_sqlite.go. (1) runDBMigrate did Status, Connect, Close, Status — three separate opens with no lock held across them, so the reported before and after could describe states other than the one migrated, and the third open added a way for a command that had already succeeded to return an error. (2) databasePath used storage.NewOsFS() where db_postgres.go uses storage.NewSafeFS(storage.NewOsFS()), an unexplained divergence in a path-handling call.
severity: minor
resolution: '(1) The after value is now the SchemaVersion constant rather than a re-read: Connect succeeded, therefore the database is at that version. One fewer open, one fewer TOCTOU window. (2) databasePath now wraps in SafeFS, matching the postgres command. Also fixed the stale Options.MaxOpenConns doc, which claimed values below 2 are raised to 2 when they are raised to defaultMaxOpenConns (8), and moved the ladder''s test accessor from an exported MigrationCount to export_test.go as MigrationSteps, which additionally lets the test assert step ordering rather than only arity.'
status: addressed
---
