---
id: RR-H8AV64
type: review-response
title: The new focus rule needs a field-to-element mapping the composer does not have
finding: '`select` now focuses the first input, textarea, select or button under `.rl-channel-composer__fields` (RlChannelComposer.vue:412-419). AC 8 needs "the first rendered field that is required and empty", so the composer must find the element for a property name. RlChannelField exposes no such hook today, and enum or ref widgets put a button first. Name the mechanism in the plan, for example `data-field` on each `.rl-channel-composer__field` wrapper and a query inside it. Cover an enum and a ref field in the story.'
severity: minor
resolution: ADDRESSED Each field wrapper carries data-field=<name>; focus goes to the first focusable element inside it, one helper shared with select(type). ComposeModeFocus covers an enum and a ref field (AC 8, Approach 8).
status: addressed
---
