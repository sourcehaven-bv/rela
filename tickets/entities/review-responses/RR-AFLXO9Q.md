---
id: RR-AFLXO9Q
type: review-response
title: Tie-break string comparison differs from Go in edge cases
finding: JS < compares UTF-16 units, Go compares bytes; number formatting of very large values differs.
severity: minor
reason: Only affects characters above U+FFFF versus U+E000-U+FFFF and integers beyond 1e21 among equally counted undeclared values; not worth extra code.
status: wont-fix
---
