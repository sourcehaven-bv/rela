---
id: TKT-HJ5Z9Y
type: ticket
title: 'Live views 2/3: per-subscription SSE frames via poke and gated re-read'
kind: enhancement
priority: high
effort: l
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

## Description

Second slice of FEAT-GWF8YH phase 1. Clients subscribe to an entity, a list or a
view. On a change the server pokes affected subscriptions and re-reads them
through `visibility` for the subscriber, batched per principal, and sends frames
that may name entity ids because every id comes from the gated re-read. Cost is
pinned with a budget test (collection-reads rule).
