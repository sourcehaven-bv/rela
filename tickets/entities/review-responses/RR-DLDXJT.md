---
id: RR-DLDXJT
type: review-response
title: Sidebar refetch too frequent; budget tests miss hot path
finding: GET /_piles resolves every item and refetched on every route change; only GET /_piles/{id} had a budget test.
severity: minor
resolution: 'Plan: usePiles refetches on its own mutations and debounced entity:changed only. Counting budget tests for GET /_piles, pile _position and export too.'
status: addressed
---
