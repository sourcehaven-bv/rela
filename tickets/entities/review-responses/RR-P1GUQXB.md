---
id: RR-P1GUQXB
type: review-response
title: Event index must not duplicate matches
finding: The rename and copy-only rules are easy to get wrong in a hand-built index.
severity: minor
resolution: 'Plan: the index is a superset prefilter by event type. One selects(trigger, event) function stays the authority for the event keys.'
status: addressed
---
