---
id: RR-2M10L2A
type: review-response
title: Change markers spliced into link, image and code markup
finding: Markers were spliced into link, image and code markup, and U+E000-E003 in input became real tags.
severity: significant
resolution: Blocks with links, URLs, code spans or tags fall back to removed plus added; marker characters are stripped from input.
status: addressed
---
