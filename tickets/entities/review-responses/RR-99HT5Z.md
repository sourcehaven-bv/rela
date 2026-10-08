---
id: RR-99HT5Z
type: review-response
title: Short start endpoint matches the tail of an unrelated paragraph
finding: A 5-rune heading passed the 0.6 floor at two edits, so 'Risks' matched 'tasks' and the start landed mid-word after the heading was renamed.
severity: significant
resolution: Endpoints under 12 runes must match exactly (crossBlockExactBelow); the aligner prefers the longer match on ties. Pinned by the 'short heading renamed' case.
status: addressed
---
