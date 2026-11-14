---
id: RR-P3PFEJP
type: review-response
title: Relation keys were not normalized
finding: A target key was compared as raw text while entity keys were converted.
severity: minor
resolution: targetKey converts the source key through the target key property before lookup.
status: addressed
---
