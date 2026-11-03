---
id: RR-9L5OP8
type: review-response
title: '''constructor'' as a property name is treated as a given property'
finding: 'RlChannelComposer.vue:306 and :314 use the in operator on a plain object literal (given built at useSessionDecisions.ts:466). ''constructor'' passes propertyNamePattern (internal/realtimecfg/validate.go:31), and ''constructor'' in {} is true. In compose mode such a field is hidden even when the preset does not set it, and givenRows renders ''Label: function Object()...''. Use Object.hasOwn(given, name), or build given with Object.create(null) or as a Map.'
severity: minor
resolution: "addressed\tThe composer checks given properties with an own-property test (hasOwnProperty) instead of the in operator, so 'constructor' and similar names are no longer treated as given."
status: addressed
---
