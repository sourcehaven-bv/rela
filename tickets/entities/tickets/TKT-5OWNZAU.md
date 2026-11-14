---
id: TKT-5OWNZAU
type: ticket
title: BodyWatch misses bodies read by header fallbacks
kind: chore
priority: low
effort: xs
status: backlog
---

## Description

`storetest.BodyWatch.ListEntityHeaders` and `BodyWatch.GraphQueryHeaders`
forward to `store.ListEntityHeaders(ctx, b.Store, q)` and
`store.GraphQueryHeaders(ctx, b.Store, q)`. When the wrapped store has no native
capability, the fallback drains `b.Store.ListEntities` or `b.Store.GraphQuery`
directly, so BodyWatch never sees the bodies the fallback reads. The comment on
`GraphQueryHeaders` says those bodies are counted; they are not.

Route the fallbacks through the decorator, as `BodyWatch.ListEntityHeadersPage`
does since TKT-Q918DFR (`headerPageOf` checks the next layer and
`store.Innermost`), and correct the comment. Found in the review of TKT-Q918DFR.
