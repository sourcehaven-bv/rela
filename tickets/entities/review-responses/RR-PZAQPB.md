---
id: RR-PZAQPB
type: review-response
title: Drop SSE id:/Last-Event-ID resume entirely — cross-process reconnect can't replay; SPA reconnect re-snapshot is the (gated) resume
finding: Broker is per-process in-memory; a reconnect can land on a different process via the LB with none of the prior history. pgstore seq is discarded in the listener (listener.go:276); store.Event has no seq, so no monotonic cursor exists at the SSE layer even within one process. Any emitted Last-Event-ID is un-replayable on the next process and adds attack surface (echoed ids the new principal may not see). Don't build it. The SPA already does new EventSource + invalidateQueries on reconnect (useEvents.ts:104) — a full GATED re-fetch. Document 'reconnect = gated re-snapshot, no event replay.'
severity: minor
resolution: Accepted — no id:/Last-Event-ID resume. Listed in ticket out-of-scope. cacheIds are stable per-principal (derived from master secret + principal_id), so the client's cacheId→entity map survives reconnect without server replay; the SPA's existing reconnect re-fetch (gated) reconciles any gap. Cross-process reconnect needs no SSE cursor state.
status: addressed
---
