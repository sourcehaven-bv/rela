---
id: RR-6NGPLV
type: review-response
title: Regression test can race listener startup
finding: With catch-up disabled, a write committed before b's listener primes and LISTENs is never delivered, so the test can time out under load.
severity: minor
resolution: Catch-up stays on at 200ms in this test; both delivery paths run in the listener, so either proves it connected. Ran 3x with -race.
status: addressed
---
