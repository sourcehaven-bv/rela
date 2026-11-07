---
id: RR-0O0O8GM
type: review-response
title: Exhaust can stay bound to an ended save
finding: exhausting clears only when left reaches 0; a bound save that ends early would block later exhausts.
severity: minor
resolution: runWrite releases the binding in a finally when its save ends.
status: addressed
---
