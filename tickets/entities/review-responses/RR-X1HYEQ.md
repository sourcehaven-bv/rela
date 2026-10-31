---
id: RR-X1HYEQ
type: review-response
title: Datetime given values show as raw input strings
finding: givenText (RlChannelComposer.vue:318) shows fromPropValue output, which for a datetime is a browser-local 2026-11-01T09:00 string. It ignores the composer's timeZoneLabel and the formatting used elsewhere. Format date and datetime values for display.
severity: nit
resolution: "addressed\tA datetime given value now shows as \"2026-11-01 09:00 (zone)\", using the composer's timeZoneLabel. Pinned by the ComposeModeDatetime story. Date values stay in ISO form."
status: addressed
---
