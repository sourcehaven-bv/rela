---
id: RR-NAOG9H
type: review-response
title: 'Rolling deploy: old and new binaries use different migrate/write lock keys'
finding: Changing a lock-key derivation means that during a rolling deploy, old and new processes on the SAME schema compute DIFFERENT keys and are therefore not mutually excluded. pgstore.Open calls Migrate on every store open, so every starting process is a migrator, and migrations are not individually idempotent (bare CREATE TABLE, ALTER TABLE DROP CONSTRAINT).
severity: significant
resolution: 'Documented rather than redesigned, and the exposure is smaller than first assessed. The write lock is belt-and-braces over work that is already transactional with row-level locking, so a brief mixed window degrades isolation rather than correctness. For migrate, the whole thing runs in ONE transaction and PostgreSQL''s own catalog locks serialize concurrent DDL, so the loser rolls back atomically — a failed startup needing a restart, never a corrupted schema — and only on a release that changes lock keys AND adds a migration together. This branch adds no migration. Note the same one-time window already occurred for the sweep lock in #1217, so this is the second and last such transition.'
status: addressed
---

Assessed per lock rather than treating "different keys during deploy" as one
issue:

- **Write Tx** — not a correctness problem. `Tx` serialization sits over work
that is already transactional with row-level locking; briefly losing
cross-process serialization degrades isolation, not committed rows.
- **Migrate** — the sharper one, and the reason for the docs note. Loud and
atomic: one transaction, catalog locks serialize the DDL, loser rolls back.

Rejected a dual-key transition scheme (taking both old and new keys during a
deprecation release) as over-engineering for a failure mode that is loud, atomic
and self-correcting.

**Why no docs note in the end.** The first draft of this fix added an upgrade
paragraph to the postgres guide. It was dropped when the change was rebased onto
#1217: that PR had already shifted the sweep lock's key space without an upgrade
note, so a note attached only to the migrate/write transition would imply the
sweep transition was safer than it was. The realistic exposure — a startup that
fails loudly and succeeds on retry, only when a release changes keys and adds a
migration together — does not warrant permanent guide text. Recorded here
instead, where it is discoverable from the bug.
