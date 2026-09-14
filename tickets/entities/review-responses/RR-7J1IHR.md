---
id: RR-7J1IHR
type: review-response
title: AC-2 'identical payload' and AC-3 'query string intact' are not assertable as written
finding: AC-2 asks two separate tests to assert payload identity, which can only be equality against a duplicated literal that drifts — drive both buttons in ONE test against the same filled form and expect(callA).toEqual(callB). AC-3 asserts the query string is intact while AC-9 asserts ?step= changes; both cannot hold on a multi-step form. Reword AC-3 to 'path unchanged, router.push not called'.
severity: minor
status: addressed
resolution: >-
  AC-2 now drives both buttons in ONE test and compares the two calls, instead of two tests comparing duplicated literals. AC-3 reworded to 'path unchanged, router.push not called'.
---
