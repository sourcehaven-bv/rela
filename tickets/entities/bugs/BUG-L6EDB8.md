---
id: BUG-L6EDB8
type: bug
title: Corpus round-trip test exceeds its 180 s timeout
description: serializerContract.test.ts sweeps every entity body in one test with a 180 s timeout. The corpus grows with every ticket; on CI the sweep reached 178 s and then failed Frontend on develop and every PR from 2026-10-09 10:01 UTC.
priority: critical
why1: The corpus sweep runs all 6,800 bodies in one test with a fixed 180 s timeout, and CI reached it.
why2: The corpus grows with every ticket and review response, so the test's runtime grows without bound while the timeout is fixed.
why3: Each body was parsed four times (twice in roundTrip, twice more for the semantic comparison), doubling the cost per body.
why4: The timeout was sized once for the corpus at the time and nothing warns as runtime approaches it.
why5: Tests over a growing repository corpus have no per-unit bound; one test owns the whole set.
prevention: One test per entity-type directory, so each test's runtime is bounded by one type, not the whole repository; parse work halved.
started: "2026-10-09"
completed: "2026-10-09"
status: done
severity: high
---

## Description

`frontend/src/components/forms/milkdown/serializerContract.test.ts` checks every
entity body in `tickets/entities` and `docs-project` in one `it` with a 180 s
timeout. CI runs the full sweep (`RELA_FULL_CORPUS=1`). The last green develop
run took 178 s; since 2026-10-09 10:01 UTC the Frontend job times out on develop
and on every PR.

## Fix

- Parse each body once per round instead of twice: reuse the parse trees
for the semantic comparison. Local full sweep: 137 s to 69 s.
- One test per entity-type directory, 120 s each. The largest group
(planning-checklists) takes 26 s locally.
