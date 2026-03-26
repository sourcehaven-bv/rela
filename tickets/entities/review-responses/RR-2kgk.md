---
finding: Setting id_caps on sequential/manual ID types is accepted but has no effect. This is confusing for users.
id: RR-2kgk
resolution: Added warning in validateEntitySemantics when id_caps is set on non-short ID types. Added TestParse_IDCapsOnNonShortType test covering sequential, manual, and short ID types.
severity: significant
status: addressed
title: id_caps silently ignored for non-short ID types
type: review-response
---
