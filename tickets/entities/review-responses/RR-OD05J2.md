---
id: RR-OD05J2
type: review-response
title: Validate item text in the engine
finding: The plan rejects only newlines and validates in the handler.
severity: minor
resolution: 'Plan: TaskItemConverter validates: trimmed, non-empty, at most 1000 runes, no C0 controls, DEL, U+2028 or U+2029; target max_length still applies.'
status: addressed
---
