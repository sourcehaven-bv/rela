---
id: RR-RGG00
type: review-response
title: Parity test (AC4 drift guard) used a fixture structurally unable to expose drift
finding: 'TestPerformable_MatchesEnforceUpdate reused snapshotMeta (only when: is value-independent count_relations) and only varied guard/counts. Both critical divergences (entity.value precondition, self-loop) passed clean. A drift guard that green-lights live divergences is decoration.'
severity: significant
resolution: 'Added driftMeta with a value-dependent `when: entity.value == to` edge and an a->a self-loop; the parity test now runs both snapshotMeta and driftMeta and asserts Performable.Allowed == (EnforceUpdate==nil) plus that self-loops are omitted-but-no-op. Both criticals would now be red pre-fix.'
status: addressed
---

## Finding

The AC4 parity test asserted read==write on `snapshotMeta`, whose only `when:`
is value-independent and which has no self-loop — so it structurally could not
catch either critical. A drift guard must be run on a fixture *capable* of
drifting.

## Fix

Reworked the test to run over `snapshotMeta` (graph precondition) **and**
`driftMeta` (value-dependent `when:` + self-loop). Asserts `Performable.Allowed
== (EnforceUpdate == nil)` for every verdict, and that self-loops are omitted
from verdicts yet no-op on write.
