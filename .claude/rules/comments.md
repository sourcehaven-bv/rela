---
paths:
  - "internal/comments/**"
---

# Comments storage

- **Comments are backend-selected, and deliberately NOT in the graph**
  (TKT-OGTVJW). `comments.Store` has three implementations — `filecomments`
  (fs/memory/desktop), `pgcomments`, `sqlitecomments` — chosen by the RECIPE,
  which passes one into `buildComments` via `backendOverrides.commentStore`; nil
  selects the file backend. Any new one must pass
  `internal/comments/commentstest.RunAll`.

  The two database backends were split along the SAME reasoning that divided
  `state.KV` from versioning, and the split is the point. **postgres**: a
  comment posted through one `rela-server` node was invisible to the others, the
  defect class TKT-VC27L3 fixed for `state.KV`. **sqlite** is single-process, so
  that argument does not apply and the decision went the other way anyway —
  commentary is content ABOUT content, so it must travel with `rela.db` the way
  versioning does (TKT-4NU9ZD), not stay node-local the way the render cache
  does (TKT-L1A3PH). "Is this about the machine or about the content?" is the
  question to ask of the next such table.

  Neither backend may import `internal/store` (arch-lint enforces it): each
  declares its own narrow `DBTX` and takes an injected handle, so comments are a
  third and fourth consumer of the one pool, never a store concern. Comments
  stay outside the graph entirely — no entity type, no audit log, no versioning,
  no search indexing (TKT-FIO205, permanently decided).
