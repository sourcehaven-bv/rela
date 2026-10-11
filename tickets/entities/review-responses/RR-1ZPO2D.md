---
id: RR-1ZPO2D
type: review-response
title: D1 retag path untested
finding: The sync loop test asserts entity state only; it passes if retag never fires.
severity: significant
resolution: 'Sync loop records the tagged token and requireBaseAt checks the tag points at it, including the retag after a create-tag conflict. Test: TestSyncLoop_ReachesFixedPoint.'
status: addressed
---
