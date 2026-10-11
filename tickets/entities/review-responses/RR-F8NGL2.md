---
id: RR-F8NGL2
type: review-response
title: Config writes were not audited
finding: Only OpDataMigration exists (audit/audit.go:148); a save without a migration left no record.
severity: significant
resolution: 'Plan changed: new audit op config-edit with principal, before/after sha256 per file, changed paths and migration name (never content), written before the rebuild; refused attempts (403/422) are logged.'
status: addressed
---
