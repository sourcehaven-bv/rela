---
id: RR-Z9TF4KU
type: review-response
title: No faced-type or ingest-level budget test for FirstEqual
finding: FirstEqual tests use a faceless type in the trivial world and there is no AssertScaleFree through ingest.Run.
severity: minor
resolution: No change.
reason: Faces and worlds run through the same gated query as HolderLookup.Matching, whose tests cover them; TestHolderLookup_FirstEqualCostIsScaleFree pins the find plus the GetAddress read that ingest makes.
status: wont-fix
---
