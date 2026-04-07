---
id: RR-1KSJ
type: review-response
title: Multi-block list items not supported
finding: Goldmark list items can contain nested lists, code blocks, and multiple paragraphs. Current extractListItems only handles single-paragraph items. Plan inherits this limitation without acknowledging it. Multi-paragraph or code-block-containing task items will lose content on round-trip.
severity: significant
resolution: Documented limitation in plan. Added test case verifying graceful handling (no crash) for multi-block items. Limitation matches existing list behavior.
status: addressed
---
