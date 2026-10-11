---
id: RR-OG0D72
type: review-response
title: Tombstones unbounded on fs
finding: 'Security design review: per-entity tombstones grow the document without limit.'
severity: minor
resolution: 'Plan: store-wide token counter max(counter+1, now µs); no tombstones.'
status: addressed
---
