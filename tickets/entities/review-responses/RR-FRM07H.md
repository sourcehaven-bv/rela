---
id: RR-FRM07H
type: review-response
title: 404 sweeps in non-denials, and the godoc claimed the ambiguity was free
finding: 'resolveAnchoredDocument produces 404 from three causes: a dropped data-entry.yaml key (code document_not_found, explicitly NOT confidential under rela''s config-is-not-a-secret rule), the ACL read gate, and a post-gate GetEntity miss. Only the second is a revocation. The godoc and the bug entity both claimed the ambiguity ''costs nothing here'', which overstates it — the config case is distinguishable by error code and is not a denial by rela''s own doctrine.'
severity: significant
resolution: 'Kept the behaviour, corrected the claim. Narrowing on the error code was considered and rejected: a dropped config key means the document genuinely no longer exists, so the empty state is the correct rendering for it, and a branch whose only effect would be keeping content visible for a deleted document buys nothing. The godoc now states plainly that two non-denials are swept in knowingly, names both, and explains why clearing is right for them anyway — the cost being an empty state rather than a wrong decision. It also now cites resolveAnchoredDocument by path.'
status: addressed
---

The reviewer's correction was about honesty in the comment rather than the
behaviour, and that is the right emphasis. "The ambiguity is free" invites the
next reader to build on a property that was never checked; "two non-denials are
swept in, here is why that is acceptable for each" can be re-evaluated when one
of those causes changes.
