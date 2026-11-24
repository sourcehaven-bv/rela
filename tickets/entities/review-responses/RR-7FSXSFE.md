---
id: RR-7FSXSFE
type: review-response
title: Changed-lines gating has no mechanism
finding: The plan says gate only new and changed lines, but ESLint has no new-from-rev and a diff-based gate is fragile in the merge queue.
severity: significant
resolution: Use a committed baseline that may only shrink (as archguard bareref_test does). No diff logic.
status: addressed
---

The plan says gate only new and changed lines, but ESLint has no new-from-rev
and a diff-based gate is fragile in the merge queue.
