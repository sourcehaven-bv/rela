---
id: RR-EU11IAA
type: review-response
title: Answer type presets disappear everywhere
finding: rowDecisions (channelView.ts:589) drops every view of the answer type, presets included; with the header gone they would have no web path.
severity: significant
resolution: 'Plan 1: rowDecisions drops only the type button (key equals messageType) covered by choices; presets stay on the row, also on the inbox row (AC 3).'
status: addressed
---
