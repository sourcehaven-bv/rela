---
id: RR-9BB3EZ
type: review-response
title: Test gaps
finding: 'No tests for inline values on type: string, nil root, several bad literals, numeric literals.'
severity: minor
resolution: 'Added: string with inline values, nil root and coerced int literals in ResultLiterals tests, all bad literals reported. A nil branch cannot compile (walker rejects it).'
status: addressed
---
