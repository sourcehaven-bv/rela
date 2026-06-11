---
id: RR-036SN
type: review-response
title: supportedPropertyTypes will reject configs that work today
finding: Go validator only constrains relation widgets, not property widgets. Today widget:checkbox on a string property works via fallthrough. Hard enforcement via supportedPropertyTypes will start rejecting existing configs. 'Same widget names' is necessary but not sufficient -- need 'same (propertyType, widget) pairs'.
severity: significant
resolution: 'Plan revised: supportedPropertyTypes is advisory only in this ticket. Mismatched (propertyType, widget) pairs log console.warn but render. Snapshot test enumerates existing repo configs to verify no regression. Tightening to error is a follow-up gated on a config audit.'
status: addressed
---
