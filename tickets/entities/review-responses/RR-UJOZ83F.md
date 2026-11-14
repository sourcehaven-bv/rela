---
id: RR-UJOZ83F
type: review-response
title: An unexpected HTTP call replaced an earlier failed assertion
finding: runCase let the transport's failure override w.failure unconditionally, so an assertion that failed before an unmatched call was lost, and the HTTP failure had no location.
severity: significant
resolution: 'world.fail promotes an HTTP failure only when it came first; runCase uses it only when no assertion failed. The transport records the test file line that was running. Tests: an earlier assertion wins; the unmatched case names its line.'
status: addressed
---
