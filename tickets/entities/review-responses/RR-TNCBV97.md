---
id: RR-TNCBV97
type: review-response
title: 'Code: faced types are untested'
finding: No test covers a faced type or a face allowlist; an address that does not round-trip would be dropped silently.
severity: significant
resolution: TestElements_Faces covers a faced type with and without a face allowlist and checks the stored path equals the recomputed path.
status: addressed
---
