---
id: RR-JBEMX6
type: review-response
title: PostgreSQL backfill under the migration lock
finding: Backfilling every blob inside the startup migration holds the advisory lock; NOT NULL DEFAULT '' hides a writer that forgets the column.
severity: significant
resolution: 'Plan: sha256 is a generated stored column computed from bytes by PostgreSQL (11+). Every writer is covered. Adding it rewrites the table once at upgrade; noted in the guide.'
status: addressed
---
