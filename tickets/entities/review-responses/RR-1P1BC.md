---
id: RR-1P1BC
type: review-response
title: Server refuses to start on unmigrated config; SPA-fallback plan is unreachable
finding: 'internal/dataentry/app.go bails with migration.Error when Detect() returns true, telling user to run `rela migrate`. So the SPA never sees an unmigrated config in practice. The plan''s ''list-level wins as runtime fallback'' is dead code. Either: (a) don''t register this in the migration system (treat as opt-in, leave server starting), and SPA falls back; or (b) accept that server-blocks-on-old-config is the contract and drop the SPA fallback logic. Cannot have both.'
severity: critical
resolution: 'Dropped SPA-side back-compat fallback. Migration is mandatory (matches existing rela-server contract: bails on detected migrations). Frontend assumes migrated config; lists.<id>.detail_view stays parseable in Go but is unused post-migration. Cleaner architecture, no dead code paths.'
status: addressed
---
