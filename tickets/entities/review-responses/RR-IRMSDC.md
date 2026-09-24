---
id: RR-IRMSDC
type: review-response
title: sqlite db reconcile --dry-run creates and migrates the database
finding: sqlitedb.Open creates rela.db and runs the migration ladder, so a dry run on a fresh checkout created a database, reported every index as would-create and exited 1, and on an older database raised its schema version.
severity: significant
resolution: 'A dry run now stats the file and reads the version read-only first: no database returns appbuild.ErrNoDatabase (CLI prints a note, exit 0), an older schema is refused with a pointer to rela db migrate. Pinned by TestSQLiteReconcileDryRun.'
status: addressed
---
