---
id: RR-VGSVN2
type: review-response
title: Line merge could write a file that differs from the checked tree
finding: keepLayout pairs lines by text and could misplace lines at the wrong depth for a file whose indentation differs from the encoder's (four-space indent, indentless lists), dropping or moving sections.
severity: critical
resolution: Apply keeps the merged text only when it parses back to exactly the checked tree; otherwise it writes the plain encoding. TestApply_WritesWhatWasChecked.
status: addressed
---
