---
id: RR-4IUY13
type: review-response
title: Patch records and audit file grow unbounded
finding: 'Reply and SetAction patches append to the parent segment without a cap or compaction; appendAudit reads the whole audit file to check the torn tail. Fix: compact past a patch threshold; read only the tail for the torn check.'
severity: minor
resolution: Segments with 2x SegmentSize patches are compacted on the next patch; audit torn-tail check reads only the last byte.
status: addressed
---
