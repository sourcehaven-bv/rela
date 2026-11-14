---
id: RR-QLY1OMP
type: review-response
title: expect_generation breaks every batch that moves the session
finding: admitPost checks expect per append; after message 1 moves the session, message 2 with the same expect is refused.
severity: significant
resolution: 'Plan: one expect_generation per batch, checked against the pre-batch session only.'
status: addressed
---
