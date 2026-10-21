---
id: RR-HBRLU2
type: review-response
title: Item text admits invisible control characters
finding: C1 controls, bidi overrides and zero-width characters pass validTaskItemText and reach titles.
severity: minor
resolution: Item text now refuses control characters, bidi controls and the line and paragraph separators. The zero-width joiner stays allowed for emoji sequences. Pinned by the RefusesBadText cases.
status: addressed
---
