---
id: RR-WGL2WR
type: review-response
title: No test pins the owning rule with the flag on
finding: The old fallback carried explicit owning handling; nothing shows a tolerated mismatch still respects ownership.
severity: minor
resolution: 'Test case: a mismatched owning edge with the flag set, target already owned, fails with ErrOwningRule and writes no audit record.'
status: addressed
---
