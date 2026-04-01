---
finding: valueToNode has an inefficient marshal/unmarshal round-trip that can corrupt types. Should use yaml.Node.Encode() instead.
id: RR-MWZR
resolution: Replaced marshal/unmarshal with yaml.Node.Encode() for type-safe conversion
severity: critical
status: addressed
title: valueToNode uses inefficient marshal/unmarshal
type: review-response
---
