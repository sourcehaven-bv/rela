---
id: RR-14HP4KA
type: review-response
title: One range can exceed 256 type names
finding: The initial entity span and a relation span with its endpoint types can exceed the server cap of 256 names in one range; packing cannot fix that.
severity: significant
resolution: initialSpans chunks entity and relation types so every range stays under 256 type names; scopes that cannot fit are refused. TestDiff_InitialSpansStayUnderTypeNameCap.
status: addressed
---
