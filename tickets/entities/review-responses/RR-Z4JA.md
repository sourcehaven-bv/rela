---
id: RR-Z4JA
type: review-response
title: Badge-sum test could derive equality instead of asserting magic numbers
finding: 'The ''summary badge total equals sum of all six check counts'' test checks errors=2, warnings=4, sum=6 as three independent magic-number asserts. The ticket invariant is ''badge total = sum of card counts''; derive that directly: expect(badgeErrors + badgeWarnings).toBe(sumOfCardCounts). Magic numbers can all be wrong in concert; the derived form can''t.'
severity: nit
resolution: Test now derives both badge total (parsed from .badge.error and .badge.warning) and card sum (parsed from .check-card .check-count) from the rendered DOM, then asserts they are equal. A final sanity check confirms the sum matches the seeded issue count. Removed the three independent magic-number asserts.
status: addressed
---
