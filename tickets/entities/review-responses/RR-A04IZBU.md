---
id: RR-A04IZBU
type: review-response
title: Nothing checks the baseline only goes down
finding: The update flag writes higher counts and the JSON diff is easy to rubber-stamp.
severity: significant
resolution: 'Added an against-ref check in CI for PRs: fails on a new file or a higher count in either baseline.'
status: addressed
---

The update flag writes higher counts and the JSON diff is easy to rubber-stamp.
