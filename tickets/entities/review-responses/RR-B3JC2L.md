---
id: RR-B3JC2L
type: review-response
title: README claimed ACL-redacted fields never reach the trace
finding: Property values passed as any (e.g. pgstore normalizeJSONNumbers v=Number("110000")) were recorded before redaction, contradicting the README.
severity: significant
resolution: Parameters and results declared as any are wrapped in Opaque and recorded as type only. README now states traces hold unredacted data and must be handled like a database dump.
status: addressed
---
