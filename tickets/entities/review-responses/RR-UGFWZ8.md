---
id: RR-UGFWZ8
type: review-response
title: 'Coverage gaps: fsstore and conformance edges'
finding: applyFaceMove ran only on memstore and the conformance test seeded only a zero-tail edge.
severity: minor
resolution: The recovery test runs on memstore and fsstore; the conformance subtest also asserts a draft-tail and an incoming edge survive.
status: addressed
---
