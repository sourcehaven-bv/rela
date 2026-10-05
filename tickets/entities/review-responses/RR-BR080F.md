---
id: RR-BR080F
type: review-response
title: Family on a named-face ref is entity level
finding: '[security] ResolvedHeader.Family is true for ID@face when another face is readable; a future consumer could read it as the face being readable and list a hidden face''s id.'
severity: nit
resolution: The Family field comment states it answers for the bare id only and that Served is the face-level answer. convert.go already requires Served for a faced tail.
status: addressed
---
