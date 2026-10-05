---
id: RR-F1XMQ2
type: review-response
title: sqlite ladder file locations are wrong
finding: Section 4.3 places the ladder in sqlitestore but schemaVersion and schemaSQL live in internal/sqlitedb. An existing explain test names entities_type_idx.
severity: minor
resolution: 'Amendment A10: names internal/sqlitedb/migrate.go and sqlitedb.go and the explain test to update in PR 1.'
status: addressed
---

## Finding

Section 4.3 places the ladder in sqlitestore but schemaVersion and schemaSQL
live in internal/sqlitedb. An existing explain test names entities_type_idx.

Design: `.ignored/stage2-design.md` section 11.
