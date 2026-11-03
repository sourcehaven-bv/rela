---
id: RR-7IA7PD
type: review-response
title: Inconsistent nil-schema guard
finding: list guarded a nil schema while RunAction did not.
severity: minor
resolution: Dropped the guard in list; the schema is always published.
status: addressed
---
