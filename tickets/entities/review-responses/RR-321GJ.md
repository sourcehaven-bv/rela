---
id: RR-321GJ
type: review-response
title: Bare URL emission breaks for URLs with spaces or unbalanced parens
finding: renderLinkInline and renderImageInline emitted the URL bare without checking for chars that require angle-bracket wrapping per CommonMark.
severity: critical
resolution: Added needsAngleBrackets and writeLinkURLAndTitle helpers. URLs with whitespace, control chars, or unbalanced parens are wrapped in `<...>`. Both link and image renderers route through writeLinkURLAndTitle (also dedups the title-emitting branch the reviewer flagged).
status: addressed
---
