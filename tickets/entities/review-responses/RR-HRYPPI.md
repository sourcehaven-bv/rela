---
id: RR-HRYPPI
type: review-response
title: realtime.Service.Describe leaks realtimecfg types
finding: 'describe.go MessageTypeDescription.Properties is []realtimecfg.Property, and ChannelTypeView carries realtimecfg.Anchor/Cardinality/Persist. dataentry reads their fields without importing realtimecfg, so D1 (''dataentry never imports realtimecfg'') holds only literally: any realtimecfg.Property field change now ripples into the HTTP mapping. The root CLAUDE.md rule is to return domain DTOs, not parsing types. Add a realtime.PropertyDescription value type (name, label, type string, required, max, enum, values, to) and convert in describeMessageType.'
severity: minor
resolution: Describe now returns plain-value DTOs including a new realtime.PropertyDescription, so dataentry no longer sees realtimecfg types.
status: addressed
---
