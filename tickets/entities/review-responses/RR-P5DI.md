---
id: RR-P5DI
type: review-response
title: Sealed-method naming inconsistent across Value, Type, IR node
finding: value.go uses predicateValue(); env.go uses predicateType(); ir.go uses nodeMarker(). Three patterns for the same sealing trick. Normalize to one — either predicateValue/predicateType/predicateNode or all-marker. Reader-facing clarity; mechanical change.
severity: minor
resolution: 'Normalized sealed-method naming: predicateValue → sealedValue, predicateType → sealedType, nodeMarker → sealedNode. All three follow the same `sealed<Domain>` pattern across value.go, env.go, and ir.go.'
status: addressed
---
