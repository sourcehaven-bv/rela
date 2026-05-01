---
id: RR-UU0I5
type: review-response
title: Tighten remove-test sanity check
finding: Sanity check at e2e/tests/reverse-relations.spec.ts:118-119 only asserts the seed contains FEAT-001 — if the seed silently grows a second implements edge, the remove test would still pass while only deleting one of two. Use toHaveLength(1) for tighter coverage.
severity: minor
resolution: Sanity check now uses toHaveLength(1) plus an explicit equality on the first element's id, so any silent fixture drift surfaces as a failure before the delete-assertion can give a false positive.
status: addressed
---
