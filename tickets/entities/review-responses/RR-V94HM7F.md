---
id: RR-V94HM7F
type: review-response
title: No markdown-package round-trip test for a --- line
finding: The fix is pinned by TestSplit and the fsstore fuzz seed; internal/markdown has no write-then-read test for a value with a --- line.
severity: minor
reason: internal/markdown reads through the same frontmatter.Split that TestSplit pins, and the fsstore seed covers the full write and read path. A markdown-level test would duplicate that coverage.
status: deferred
---
