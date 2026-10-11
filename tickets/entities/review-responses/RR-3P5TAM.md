---
id: RR-3P5TAM
type: review-response
title: State shared across generations
finding: The on-disk search index lock and render cache are shared between generations, MCP sessions are not carried over a swap, and jobs queued by the old generation run on it.
severity: minor
reason: Rebuilt generations use an in-memory index until restart (documented); the other effects are short-lived and listed as residuals for a follow-up.
status: deferred
---
