---
id: BUG-ALPYHR
type: bug
title: SQLite force-live purge was re-captured by the sweep
description: 'On SQLite builds v26.9.7 to v26.10.4 a history-purge --force-live wrote its tombstone with a wrong content hash because liveEntityHash selected id, face, type while scanEntity reads id, type, face. The next sweep captured the purged content again. #1811 fixed the order and added a covering test. This bug adds a narrower regression test, removes the hand-written column lists and documents the operator repair. GitHub #1807.'
priority: high
effort: s
why1: liveEntityHash selected the columns in a different order than scanEntity reads them, so the tombstone hash was computed over swapped type and face.
why2: The query repeated the column list by hand instead of using getEntitySQL, which the other scanEntity callers share.
why3: 'The existing conformance case UnchangedSaveAfterForceLivePurge (added with the fix in #1811) does pin it, but no test had pinned it before #1576 shipped; the purge tests only checked that a tombstone exists, not that it suppresses a re-capture.'
why4: The purge conformance tests and the sweep tests lived in separate harnesses, so the interaction between them had no owner.
why5: Erasure correctness was asserted on the purge result rather than on the observable history after the next sweep.
prevention: Live-hash queries in both backends use the shared column constants (getEntitySQL, relationColumns, getRelationSQL), so they cannot drift from the scanners. Conformance case SweepBacklog/ForceLivePurgeIsNotRecaptured asserts the history after a force-live purge and a sweep for zero-face and faced entities and for relations.
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

## Description

GitHub #1807 (IB review of #1804).

`liveEntityHash` in `internal/store/sqlitestore/purge.go` selected `id, face,
type` while `scanEntity` reads `id, type, face`. The tombstone hash therefore
did not match the live row, and the sweep re-captured the erased content (secret
or personal data) into history.

Affected: SQLite builds v26.9.7 to v26.10.4 (since #1576). PostgreSQL is not
affected.

The column order was corrected on develop by #1811 (BUG-1DWMYO), together with a
covering conformance case. No release contains the fix yet. This bug adds a
narrower regression test, replaces the hand-written column lists in both
backends with the shared constants, and documents the operator repair.
