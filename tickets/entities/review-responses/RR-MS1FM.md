---
id: RR-MS1FM
type: review-response
title: deleteEntity swallows errors by default
finding: deleteEntity catches and ignores errors. A test verifying DELETE works would pass even if it silently failed. Let caller opt into cleanup semantics with .catch(() => {}).
severity: nit
resolution: deleteEntity no longer swallows errors. Callers doing cleanup-only deletion now must append .catch(() => {}) explicitly.
status: addressed
---
