---
id: TKT-D4EEYS
type: ticket
title: Bound the per-request cost of resolving comment anchors
kind: enhancement
priority: low
status: backlog
---

## Description

The list-comments route resolves every text anchor on the target against the
body on each read, with no time or work budget. Each resolve is bounded, but the
request is not: up to 500 comments, each falling through to the fuzzy phases on
a large body of short paragraphs, cost minutes of CPU per read (measured by the
TKT-U32AUB security review: 1.66 s per non-matching cross-block quote on a 1 MB
body). Any reader of the entity triggers it, and nothing caches the result.

## Proposed approach

- Give the resolve loop in `listComments` a per-request budget (request
context plus a cell or time budget). Anchors left unresolved when it runs out
are reported as not located for this read, distinct from detached.
- Consider caching resolved ranges per (body hash, anchor) for the read path.
