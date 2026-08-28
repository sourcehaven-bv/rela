---
id: RR-U5ICXO
type: review-response
title: 'Two mechanisms were asserted by no test at all'
finding: 'A mutation battery over the full suite showed that deleting the navigation-guard fence (if confirmingClear return false) or the formGeneration bump in loadEntity left every test passing. Both are the mechanisms the unmount and singleton-collision fixes depend on. A fence tested in a vacuum while the gate it guards swings free is the same unit-tested-but-broken-in-the-browser pattern that produced the previous failures.'
severity: significant
resolution: 'Both now have tests that fail when the mechanism is removed: the navigation fence via a captured leave-guard invoked directly, and the unmount bump via a mount/unmount/answer sequence asserting zero writes.'
status: addressed
---
