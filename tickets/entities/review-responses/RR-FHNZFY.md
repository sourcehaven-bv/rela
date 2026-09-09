---
id: RR-FHNZFY
type: review-response
title: Read-gate test could not distinguish check (1) firing from an absent source
finding: The denyReadGate sub-test asserted only that ErrCopySourceMissing came back. That same error is what readCopySource returns when the store lookup fails, and readCopySource runs before authorizeCopy, so two code paths satisfy the assertion. The current behaviour is right (the reviewer confirmed the control case with AllowAllCopyReadGate succeeds), but a future reordering that removed the check (1) call site would still pass.
severity: significant
resolution: denyReadGate now records whether PermitsReadFace was invoked, and the test asserts it was. This makes the test about the gate rather than about an error value.
status: addressed
---
