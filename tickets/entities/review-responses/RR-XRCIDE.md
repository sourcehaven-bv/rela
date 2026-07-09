---
id: RR-XRCIDE
type: review-response
title: Truncation banner counts can disagree under concurrent writes
finding: 'The banner reads ''Showing {entities.length} of {meta.total}'': the deduped merged count versus the LAST page''s total — different snapshots when a write lands mid-loop, e.g. ''Showing 4998 of 5001''. Cosmetic: the banner only appears past the ~5,000-entity cap, and its message (board incomplete) stays correct even when the numbers are slightly stale.'
severity: minor
reason: Neither operand is authoritative while writes race the loop, and no fetch strategy can make them so — the SSE invalidation that follows the racing write refreshes both. The banner's job is 'data is incomplete', which it does correctly; making the numbers transactionally consistent would add complexity for a state that should never occur.
status: wont-fix
---
