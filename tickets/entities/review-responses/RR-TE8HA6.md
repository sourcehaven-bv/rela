---
id: RR-TE8HA6
type: review-response
title: One cancelled relation fetch aborts loading of sibling relation controls
finding: 'FilterBar.vue loadRelationCandidates: on isCancelledFetch the catch does `return`, exiting the whole loop. With two relation controls A and B, a cancelled fetch for A leaves B permanently unloaded. The error-vs-cancel paths are asymmetric (error→continue via loop, cancel→return). Should be per-control try/catch so one control''s failure never affects a sibling.'
severity: significant
resolution: 'FilterBar.vue loadRelationCandidates: the cancel branch now `continue`s instead of `return`ing, and each control loads in its own try/catch, so one control''s cancellation/failure never blocks a sibling. Tested: ''a cancelled fetch for one control does not block a sibling control''.'
status: addressed
---
