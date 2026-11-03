---
id: RR-65MTAH
type: review-response
title: Optional enum shows both a dash option and a placeholder
finding: 'RlChannelField.vue:53 and 136. An optional select gets a ''—'' option and also placeholder ''Choose…'', so the empty state has two spellings. Pick one: drop the synthetic option if RlSelect''s placeholder can be reselected, or drop the placeholder.'
severity: nit
resolution: An optional enum now has only the empty option, labelled with the placeholder or a dash, and the select placeholder is used only for required enums.
status: addressed
---
