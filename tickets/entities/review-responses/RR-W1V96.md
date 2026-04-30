---
id: RR-W1V96
type: review-response
title: 'Test plan gaps: popstate guard test, dirty.value clearance assertion, repeated cancel'
finding: 'Add: (a) popstate-driven navigation test (window.history.back) — distinguishes await-in-guard from next(false)+push; (b) assert dirty.value is false after user accepts the leave prompt; (c) test that user can cancel twice in a row (state recovers). Drop the unmount test if not honestly testable per finding above, or do it via a host-component mount.'
severity: minor
resolution: 'Test plan now includes: (a) popstate via window.history.back; (b) assertion that dirty.value is false after user accepts leave; (c) cancel-twice-in-a-row state-recovery test; (d) honest unmount test via host-component mount.'
status: addressed
---
