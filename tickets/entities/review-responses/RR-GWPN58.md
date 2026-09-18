---
id: RR-GWPN58
type: review-response
title: Every faced type merged into one finding whose example cap could hide a type
finding: 'The aggregation key was entity.Face, and every bare row carries the SAME zero face, so bare rows across all faced types landed in one ptrAgg with an empty Subject and a five-example cap. On the perf corpus that was 35 rows across `policy` and `document` in a single finding whose examples were all DOC-*, leaving `policy` entirely invisible. The operator fixes what they can see, re-runs, and discovers a type they were never told about. Worse than for undeclared-face, where the Subject at least names the thing: here the Subject was empty, so the finding carried no information about which types were affected outside the capped example list. The remedy is per-type anyway (migrate_face takes `entity: <type>`), so one finding spanning two types described two different migration steps.'
severity: critical
resolution: 'Reproduced with three types in one project (one finding, count=5, examples spanning all three). Re-keyed the aggregation on a faultKey{status, subject} where subject is the FACE for undeclared-face and the TYPE for the two type-shaped faults, matching where each remedy applies. Now one finding per type, each naming its type and carrying its own example budget. Verified on the perf corpus: two findings (document: 20, policy: 15) where there was one. Mutation-verified: reverting the key to the face fails the test with the exact shipped shape (merged finding, empty subject, `type ""` in the sentence).'
status: addressed
---

This is the finding I would have shipped without review, and it was the more
damaging of the two: a silently incomplete report in the tool whose whole job is
to be complete.

It also resolved the dangling-colon render defect I had patched separately in
the CLI. With a non-empty Subject on every finding the formatter needs no
branch, so that patch was reverted rather than kept — the right fix made the
workaround unnecessary.
