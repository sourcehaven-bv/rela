---
id: RR-XNKLVU
type: review-response
title: Clean body merge can be dropped silently
finding: If fresh._versions is missing and mergeText succeeds with merged != theirs the content is removed from the patch and nothing is written or reported.
severity: minor
resolution: 'A clean merge that cannot be guarded (no fresh token) is now reported as a content conflict instead of being dropped. Test: ''reports a clean body merge it cannot guard rather than drop it'' (mutation-verified).'
status: addressed
---
