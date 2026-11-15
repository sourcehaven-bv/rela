---
id: RR-RRRW3K0
type: review-response
title: Type-less Visible search scans denied types
finding: With q.Types empty, Visible read every type, including types the scope denies.
severity: minor
resolution: admittedHits restricts inner.Types to the scope keys when the scope has no wildcard.
status: addressed
---
