---
id: RR-0MAK61
type: review-response
title: Set templates compile against the first attach_to type only
finding: 'internal/realtimecfg/outputs.go:121-130: props are validated per attach_to type, but o.props keeps the first type''s Templates, whose target (date vs datetime formatting, integer coercion in refValue/literalValue) is that type''s property type. An anchor of another attach_to type where the same property has another type gets values shaped for the first type. Fix: refuse a set whose property type differs between attach_to types, or keep templates per anchor type.'
severity: minor
resolution: realtimecfg refuses a set property whose type differs between attach_to types; new table row.
status: addressed
---
