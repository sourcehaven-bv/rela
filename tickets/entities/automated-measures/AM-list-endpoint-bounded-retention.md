---
id: AM-list-endpoint-bounded-retention
type: automated-measure
title: "One list request reads no body it will not render, whatever the type size"
kind: test
location: internal/dataentry/listbodies_test.go + internal/store/storetest/bodywatch.go
status: implemented
description: "A list request over a type with N large-bodied entities must not read N bodies to render one page. Pins BUG-SDMD6O. Asserted by COUNTING bodies (storetest.BodyWatch), not by measuring heap — counting is backend-independent, so it fails on memstore too, which a heap probe cannot."
---

Pins BUG-SDMD6O.

`GET /api/v1/<type>` over a type holding N entities with large bodies must
retain heap proportional to **page size**, not to N.

Measured baseline on current `develop` (pgstore, 5,000 x 20KB bodies, one
request): **+101 MB** retained to render 50 rows. Header-only retention over the
same data is **+1 MB**.

## The load-bearing part: it must not be a HEAP assertion

The original plan was a DB-gated heap test, on the reasoning that memstore
hands out entities that **share** the body string rather than materialising a
copy, so an identical heap probe reports **~1 MB whether or not the defect is
present**. That reasoning about memstore is correct. The conclusion drawn from
it — put the assertion in the pgstore suite — was not the best available one.

The instrument used instead COUNTS BODIES rather than weighing them
(`storetest.BodyWatch`, the third sibling of `Counting` and `Breadth`). "The
list pipeline was served N bodies to render 50 rows" is exactly the defect,
and it is true on memstore, fsstore and pgstore alike. That makes the pin:

- **backend-independent** — it runs in the default `go test ./...`, not only
  where `RELA_TEST_DATABASE_URL` is set (one CI job, few dev machines);
- **exact** — no threshold, no GC, no allocator-noise tolerance;
- **more sensitive** — it catches a 5-body page-sized regression that no heap
  threshold would ever separate from noise.

Mutation-verified: reverting the AllowAll branch to `ListEntities` fails it
with "list of 50 rows read 50 bodies, want 0", ON MEMSTORE — the backend the
original plan correctly identified as blind to the heap probe.

## Assertions

1. A list request reads ZERO bodies, at any type size. (Counted, not weighed —
   see above. Zero is the honest bound: served rows render properties and
   paged-out rows are not rendered at all.)
2. Byte-identical results before and after: same ids, same order, same `total`,
   across AllowAll / scope-restricted / DenyAll verdicts.
3. ACL row-gating and field redaction unchanged, asserted as **equivalence
   against the existing `ListEntities` path** rather than hand-written
   expectations — the pattern TKT-1ESTYJ used, and mutation-verified by removing
   the gate and confirming failure.
4. Callers that genuinely need bodies still receive them. `include_content=true`
   pays for the PAGE (asserted at exactly per_page, not 0 and not N), and a list
   export with an `export_render:` Lua script receives row bodies — that one had
   silently regressed to empty strings when collection reads became content-free.

## Note

The sibling risk is concurrency: N simultaneous list requests hold N copies. A
concurrency variant of assertion 1 is worth adding if it can be kept cheap —
TKT-1ESTYJ's 3-concurrent case is the precedent (6,264 MB → 55 MB).
