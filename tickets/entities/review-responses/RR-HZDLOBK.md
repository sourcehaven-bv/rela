---
id: RR-HZDLOBK
type: review-response
title: Memory queue payload copies are shallow
finding: maps.Clone copied only the top level, so a nested slice changed by the caller or by a handler reached the queued job or its retry.
severity: minor
resolution: 'Addressed: clonePayload copies nested maps and slices. TestMemoryQueue_PayloadIsCopied changes a nested slice.'
status: addressed
---
