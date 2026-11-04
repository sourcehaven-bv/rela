---
id: RR-IW9GFGJ
type: review-response
title: ChannelPresentation.placement typed as plain string
finding: The known value is not visible in the type.
severity: nit
resolution: Typed as 'above_relations' | (string & {}), so unknown values from a newer server still type-check.
status: addressed
---
