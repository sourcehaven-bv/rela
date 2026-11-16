---
id: RR-ZOU8IWQ
type: review-response
title: Threat model lacks Spoofing and test-weakening rows
finding: STRIDE incomplete; a PR can weaken or delete test cases.
severity: minor
resolution: 'Spoofing row added (N/A: no identity or run_as change); Tampering row for weakened tests mitigated by --require-tests and the trigger coverage test plus review.'
status: addressed
---
