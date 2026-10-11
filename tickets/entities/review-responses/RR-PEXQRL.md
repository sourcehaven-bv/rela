---
id: RR-PEXQRL
type: review-response
title: Rollback after a partial migration is impossible
finding: 'The runner rewrites entities in batches inside store.Tx (fs: no rollback) and advances applied.json per file (run.go:160-176). Restoring config after a partial run leaves half-migrated data.'
severity: critical
resolution: 'Plan changed: files are restored only for failures before the runner starts. After it starts the save rolls forward: files stay, the response says the migration is incomplete, and POST /_configure/migrate retries (steps are idempotent). AC11 rewritten.'
status: addressed
---
