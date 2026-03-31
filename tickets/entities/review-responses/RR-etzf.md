---
finding: Calling Relation('type').Build() creates invalid relation with empty From/To. Should either generate random IDs or panic with clear message.
id: RR-etzf
resolution: Added validation in Build() that panics if From or To are empty. Added tests TestRelation_Build_PanicsOnMissingFrom and TestRelation_Build_PanicsOnMissingTo.
severity: significant
status: addressed
title: Relation.Build() does not validate From/To
type: review-response
---
