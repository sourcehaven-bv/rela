---
id: RR-9IHU2
type: review-response
title: Prefer AbortController over sequence number for stale-request cancellation
finding: 'Plan proposes a sequence-number guard for dropping stale searchEntities responses on rapid typing. Works, but axios (used by api.get) supports AbortController natively (axios 0.22+), which actually cancels in-flight HTTP requests, freeing the server early and avoiding rendering of cancelled-but-completed responses. Required: check frontend/src/api/client.ts to see if api.get accepts an AbortSignal. If yes, use AbortController in the palette and update plan accordingly. If no (and adding it is out of scope), keep the sequence number and document why.'
severity: minor
resolution: 'Verified frontend/src/api/client.ts:49-56 — api.get already accepts an optional `signal: AbortSignal` parameter. searchEntities will be extended with an optional third arg `signal?: AbortSignal` and forward it to api.get. Plan updated to use AbortController per request, aborting the previous in-flight call before issuing a new one.'
status: addressed
---
