---
id: RR-MO6AXNR
type: review-response
title: sqliteexec.NewInTx splices an unvalidated savepoint name into SQL
finding: The savepoint name became a parameter of a shared adapter, guarded only by godoc.
severity: minor
resolution: NewInTx refuses names outside ^[a-z][a-z0-9_]{0,63}$; TestNewInTxRejectsHostileSavepoint.
status: addressed
---
