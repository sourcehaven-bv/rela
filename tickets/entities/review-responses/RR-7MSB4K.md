---
id: RR-7MSB4K
type: review-response
title: Split switch on the same err encodes a security ordering as adjacency
finding: The fix bisected attachWorld's single `switch { case duplicated / unknown / denied / err != nil }` into two switches on the same `err`, with the new refusal wedged between them. That makes `errors.Is(err, ...)` tested in two places, with nothing structural stopping a future arm being added to the wrong one — which is precisely the shape of the bug being fixed (an arm on the wrong side of a check). The security-critical ordering was encoded as adjacency plus a comment.
severity: significant
resolution: 'Applied the reviewer''s suggested shape: extracted `refuseWorldConfigError` for the two config 400s, so attachWorld now reads as two sequential guards followed by ONE switch. Errors are dispatched in one place per concern and the ordering is expressed by statement order rather than by a comment. Verified both guards are load-bearing by mutation: disabling `refuseWorldConfigError` fails TestRefuseWorldIncapablePath_DoesNotShadowConfigErrors (both subtests); disabling `refuseWorldIncapablePath` fails all five subtests of TestAttachWorld_DeniedWorldRefusedLikePermitted including the LEAK assertion.'
status: addressed
---
