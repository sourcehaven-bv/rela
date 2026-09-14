---
id: RR-UKHCVD
type: review-response
title: Cross-entity scoping test passed for the wrong reason under a blanket read-only ACL
finding: 'TestCopy_GuardDoesNotOverruleACrossEntityWrite asserted only that some *acl.ForbiddenError came back, using acl.ReadOnlyACL. That ACL denies every write with one fixed decision (RuleKind=read-only), so the test would pass identically if the exemption were widened to every guarded copy: the cross-entity write would still be refused, just by a blanket denial rather than by the scoping the test claims to pin. It could not distinguish correct scoping from an ACL that says no to everything. The fixture compounded this -- guarded-spawn targeted a new faceless entity, so sourceTail == targetTail, meaning the IsSameEntity clause was not exercised either.'
severity: significant
resolution: 'Switched to createOnlyACL against an EXISTING target, so the denial can only come from the OpUpdate check reaching the ACL, and asserted RuleKind == "test" so a refusal for a different reason fails loudly. Changed the fixture to `from: ticket` / `to: page@published` so the copy is cross-entity AND face-crossing, which is what makes the IsSameEntity clause the only thing refusing it. Verified by mutation: dropping IsSameEntity now fails this test specifically.'
status: addressed
---
