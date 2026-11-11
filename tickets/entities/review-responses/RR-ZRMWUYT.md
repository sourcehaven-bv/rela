---
id: RR-ZRMWUYT
type: review-response
title: CheckState does not check fork invariants
finding: The store accepts a forked run with a run-level pending mark or claim, an arrived branch with a mark, or duplicate generations.
severity: minor
resolution: 'Fixed in TKT-FWQZGAM: CheckState now rejects a run-level pending mark or claim while forked, a pending mark or claim on an arrived branch, and duplicate generations across the run and its branches. TestCheckState_ForkedShape and the run-store conformance suite cover it.'
reason: put is the only writer (TestEngine_OneUpdatePath) and builds only valid shapes. Store-side checks are defence in depth; deferred to TKT-FWQZGAM, which adds the second writer.
status: addressed
---
