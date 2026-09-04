---
id: TKT-9KZGJO
type: ticket
title: World-aware analysis and cross-world search grouping (Step 5)
kind: enhancement
priority: medium
effort: l
status: backlog
---

Design doc §7, §10.

- Cardinality rules declare `must_hold_in: [worlds]`; absent → default world only, so existing metamodels behave identically. Subjects and edges come from the same resolved graph; violations labeled by world (one rule can fail in one world and pass in another — both reported).
- Orphans run in the default world; states never appear in orphan output.
- Search: query scope is a world (one hit per entity within it); editor surfaces query several worlds, grouped by entity in the search API (SPA/MCP/CLI share it; `limit` counts entities). Grouping runs AFTER the per-world read gate — annotating "also matched in editorial" for someone who may not read that world is an existence-and-content oracle.

## Search — decided (Jeroen, 2026-08-28)

**A world IS the search scope.** Search does not need its own notion of which
faces to look at: a world already answers "which face of each entity do I see",
fallback chains included, per type. So searching a world means searching each
entity's resolved prime in that world.

Two examples, and they need no per-world `searchable:` config because the chains
already encode the difference:

| World | `select:` | Search behaviour |
|---|---|---|
| `published` (ISMS) | `[published, draft]` | a policy that exists only as a draft still matches, via its draft face |
| `site-nl` | `[nl, en]` | Dutch, falling back to English, and **never French** |

**Consequences that simplify the build:**

- **One hit per entity, structurally** — a world resolves to at most one prime
  per entity, so `limit` counting entities is trivially true. No `PARTITION BY`,
  no over-fetch-and-group: rows ARE entities.
- **No new operator config.** An earlier draft of this design proposed
  `searchable: true|false` per world. Unnecessary — the world's own chain says
  it.

**A fallback match must be visibly a fallback.** If a `published`-world search
matches only because the DRAFT face contains the term, the result must say so —
otherwise the reader sees a hit whose displayed (published) text does not
contain what they searched for.

Start simple: a label on the result indicating the item is a concept/fallback.
The signal already exists — it is the same `via: chain | fallback-default` the
entity response carries, so this is surfacing an existing field rather than
computing something new.

Display the resolved PRIME; label the fallback. Showing the draft's text under a
published-world search would be the other error.

## Multi-world grouping is a SEPARATE, explicit mode

The grouped view ("this text appears in both concept and published") is an
EDITOR surface deliberately querying several worlds — not the default. It is
where `PARTITION BY`/over-fetch actually applies, and it stays gated on the
read-gate rule above: a world the viewer may not read must be absent, never
summarised as "1 other match".

## Structural blocker, before any of the above

`search.Backend.Search(text string, limit int) ([]string, error)`
(`internal/search/types.go:38`) takes **no world** and returns **bare IDs**.

- Single-world search needs the world in;
- grouping needs the matched FACE out.

Both pgstore (`search.go:76`, currently pinned `AND face = ''`) and the bleve
backend must change together — `visiblesearch.go` notes the gated and ungated
streams are held to an ordered-subsequence conformance contract, so a scope
change in one without the other breaks it.

Gated on the cardinality consolidation and the Step 2 world scope.
