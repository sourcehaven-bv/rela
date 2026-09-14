---
id: RR-33CJKK
type: review-response
title: dsn() concatenates the path too, and os.Exit in runDBStatus bypasses defers
finding: Two pre-existing issues this ticket makes more visible. (1) dsn() in sqlitestore.go builds `path + "?_pragma=..."` by concatenation, so a '?' in the project path swallows the PRAGMA parameters. verifyBusyTimeout correctly catches it, but the resulting error accuses the developer of using db.Exec for PRAGMAs — an offence nobody committed — which is a genuinely misleading afternoon. (2) runDBStatus calls os.Exit(1) inside a Cobra RunE, skipping deferred cleanup and making it untestable without a subprocess; db_postgres.go:69 does the same.
severity: minor
reason: 'Both are pre-existing on develop and neither is introduced or worsened by this diff — (1) predates it and (2) matches the postgres command deliberately, since a sqlite build in the same CI pipeline must behave the same way. Fixing (2) properly means a sentinel error mapped to an exit code at the CLI root, which touches both build-tagged files plus the root command and is its own change. Filed rather than bundled: this ticket is already carrying two critical fixes, and widening it further would make the diff harder to review, not easier. The Status path — the one this ticket added — IS escaped correctly (RR-7J4A0T).'
status: deferred
---
