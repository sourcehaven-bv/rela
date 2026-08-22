---
id: AM-list-endpoint-bounded-retention
type: automated-measure
title: "One list request's heap is bounded by page size, not by the type's entity count"
kind: test
location: internal/dataentry/ + internal/store/pgstore/ (test to be written with the fix)
status: proposed
description: "A list request over a type with N large-bodied entities must not retain N bodies to render one page. Pins BUG-SDMD6O. Must run against a REAL backend — memstore shares body strings and reports the same heap either way, which is why this defect shipped."
---

Pins BUG-SDMD6O.

`GET /api/v1/<type>` over a type holding N entities with large bodies must
retain heap proportional to **page size**, not to N.

Measured baseline on current `develop` (pgstore, 5,000 x 20KB bodies, one
request): **+101 MB** retained to render 50 rows. Header-only retention over the
same data is **+1 MB**.

## The load-bearing part: it must not run on memstore

memstore hands out entities that **share** the body string rather than
materialising a copy, so the identical probe reports **~1 MB whether or not the
defect is present**. A memstore assertion here is not a weak test, it is a test
that cannot fail.

That is precisely why this shipped: the fast unit path is structurally blind to
body-retention cost. The assertion belongs in the DB-gated pgstore suite
(`RELA_TEST_DATABASE_URL`, skipped when unset), alongside the store conformance
tests.

## Assertions

1. Heap growth for one list request over 5,000 x 20KB entities stays well below
   the dataset's body size — single-digit MB, not ~100 MB.
2. Byte-identical results before and after: same ids, same order, same `total`,
   across AllowAll / scope-restricted / DenyAll verdicts.
3. ACL row-gating and field redaction unchanged, asserted as **equivalence
   against the existing `ListEntities` path** rather than hand-written
   expectations — the pattern TKT-1ESTYJ used, and mutation-verified by removing
   the gate and confirming failure.
4. Callers that genuinely need bodies (export) still receive them.

## Note

The sibling risk is concurrency: N simultaneous list requests hold N copies. A
concurrency variant of assertion 1 is worth adding if it can be kept cheap —
TKT-1ESTYJ's 3-concurrent case is the precedent (6,264 MB → 55 MB).
