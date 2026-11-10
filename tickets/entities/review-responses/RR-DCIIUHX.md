---
id: RR-DCIIUHX
type: review-response
title: os.Exit after defer release() in db status and reconcile --dry-run
finding: db_postgres.go called os.Exit(1) after defer release(), so the schema pin was only freed by process exit.
severity: minor
resolution: Both return relaerrors.NewExitError(1), which runKong maps to the exit code after the defers run.
status: addressed
---
