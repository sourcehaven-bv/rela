---
id: RR-LCM5
type: review-response
title: ctxSpyStore interface embedding hides gap if future bindings call new Store methods
finding: 'internal/lua/runtime_test.go:2266-2284: ctxSpyStore embeds store.Store and overrides only GetEntity, ListEntities, ListRelations. Any new Lua binding that calls a different Store method (e.g. CountEntities, GetRelation) will pass through silently — no record() call, no test failure. The test''s regression net is invisible-shaped to the next contributor.'
severity: significant
resolution: Added a `readStore` consumer-side interface that explicitly enumerates the three Store methods Lua read bindings invoke today; ctxSpyStore must satisfy it via a compile-time `var _ readStore = (*ctxSpyStore)(nil)` assertion. Cannot drop the embedding because runtime takes store.Store (full surface). Added a prominent doc comment on ctxSpyStore explicitly directing future contributors to add an override when a new binding starts calling a different Store method, so the gap is visible rather than implicit.
status: addressed
---
