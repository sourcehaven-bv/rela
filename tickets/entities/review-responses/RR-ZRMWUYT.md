---
id: RR-ZRMWUYT
type: review-response
title: CheckState does not check fork invariants
finding: The store accepts a forked run with a run-level pending mark or claim, an arrived branch with a mark, or duplicate generations.
severity: minor
reason: put is the only writer (TestEngine_OneUpdatePath) and builds only valid shapes. Store-side checks are defence in depth; deferred to TKT-FWQZGAM, which adds the second writer.
status: deferred
---
