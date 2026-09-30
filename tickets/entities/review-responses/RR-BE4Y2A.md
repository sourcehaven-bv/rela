---
id: RR-BE4Y2A
type: review-response
title: Face order silently changes two read paths
finding: relationTailOr404 and readableFaceOf now pick the first declared face instead of the alphabetically first one (cranky review).
severity: significant
resolution: TestFaceOrder_WorldAbsentPageUsesDeclarationOrder covers the world-absent page; relationTailOr404 uses the same newVisibleReader resolver. The change is stated in the PR description.
status: addressed
---
