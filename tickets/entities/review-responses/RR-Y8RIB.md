---
id: RR-Y8RIB
type: review-response
title: Doc on Workspace.Rename overpromised atomicity
finding: The doc claimed 'all atomically via WithTx' without mentioning the inherited rollback caveats from repository.Transaction (silent phase-2 delete failures, destructive rollbackRenamed).
severity: minor
resolution: Added an explicit caveat paragraph pointing to WithTx's docs for the inherited rollback hazards.
status: addressed
---
