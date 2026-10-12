---
id: RR-6GXZJX
type: review-response
title: AC2 and AC7 tests do not prove their claims
finding: 'Cranky review: AC2 did not check that the call blocked; AC7 used a timing window and did not test that a freed slot admits the next watch.'
severity: minor
resolution: AC2 asserts the call returned after the write delay. AC7 holds four watches open, asserts the fifth is refused, cancels one, and asserts the next is admitted.
status: addressed
---
