---
id: RR-QXC3A
type: review-response
title: useConfirm.test.ts asserts p1.toBe(p2) which tests implementation not behavior
finding: The behavioral assertion is 'both callers see the same decision', which the await lines below cover. expect(p1).toBe(p2) locks in the implementation detail that the same Promise reference is returned. Drop the line.
severity: nit
resolution: Removed expect(p1).toBe(p2) line. Renamed the test from 'returns the same in-flight promise' to 'returns the in-flight decision' to focus on observable behavior.
status: addressed
---
