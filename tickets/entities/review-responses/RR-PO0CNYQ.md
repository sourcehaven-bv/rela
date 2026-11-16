---
id: RR-PO0CNYQ
type: review-response
title: Gated NextCursor is a pre-filter more-rows signal
finding: When the page fills NextCursor is returned whenever the store has later rows even if all are hidden. A consumer that exposed it as has-more would leak that hidden edges follow a known key. No consumer exposes it today.
severity: minor
resolution: The ListRelationsPage godoc states that a non-empty NextCursor means more raw rows not a visible one and must not be shown as has-more. The cursor follow-up TKT-Q02WIFH inherits that constraint.
status: addressed
---
