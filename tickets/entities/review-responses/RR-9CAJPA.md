---
id: RR-9CAJPA
type: review-response
title: pg migration 0020 builds indexes under the table lock
finding: ADD COLUMN takes ACCESS EXCLUSIVE until the migration commits, and two CREATE INDEX statements scan the heap under it, so older nodes wait during a rolling deploy.
severity: minor
resolution: Documented in docs/postgres-backend.md. CREATE INDEX CONCURRENTLY cannot run inside the single migration transaction that pgstore.Migrate uses for every migration.
status: addressed
---
