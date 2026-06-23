---
id: RR-2LTVUL
type: review-response
title: Literal fresh-acl.Request-per-event is a DoS amplifier (O(connections×writes) member-of walks + DB queries); use event-driven re-resolve
finding: 'Per create/update event per connection, ForPrincipal→computeGlobals→walkMembers (depth-capped BFS, one ListRelations per frontier node) + Query-verdict MatchingIDs (a store/DB round-trip). M connections × W attacker-inducible write events = O(M×W) graph walks + DB queries, synchronously. 50 idle connections × 200-entity cascade = 10k walks + 10k queries. Fix (reviewer''s leverage): pumpStoreEvents ALREADY sees the EventRelation* events it currently drops (watcher.go:192) — use a member-of/role-relation edge change as the trigger to invalidate per-connection cached Requests (event-driven re-resolve), giving bounded cost AND near-instant revocation. Also: ReadQuery(type) once per (connection,window) — AllowAll/DenyAll short-circuit with zero queries; only Query pays MatchingIDs. NEVER gate inside pumpStoreEvents (serializes all connections behind the slowest gate). Confirmed: a fresh Request DOES re-walk the live StoreGraph, so staleness (RR-K2WKEJ) is genuinely fixed.'
severity: critical
resolution: 'Dissolved by the per-type reframe. The DoS amplifier was per-ENTITY gating (member-of walk + MatchingIDs per event per connection). Per-type gates on ReadQuery(type) — constant-time for AllowAll/DenyAll, no per-entity store query even for Query verdicts — resolved once per connection and cached, refreshed only on rare membership-changing events. Plus debounce coalesces bursts to one nudge per type per window. The O(connections×writes) walk is gone: O(connections) cheap verdicts per window. AC5 (debounce) + AC6 (cached cheap gate) pin it.'
status: addressed
---
