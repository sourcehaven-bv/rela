---
id: RR-LSN4FVG
type: review-response
title: Prefilter pushdown compared date equality as text
finding: PushdownPrefilters pushed prop:due=$today to the store; a YAML-decoded time.Time does not equal the text form.
severity: significant
resolution: dateOnAnyType excludes date and datetime properties from PushdownPrefilters; TestV1Search_DateVariables seeds a time.Time row.
status: addressed
---
