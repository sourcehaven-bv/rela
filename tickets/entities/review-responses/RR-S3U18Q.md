---
id: RR-S3U18Q
type: review-response
title: 'MCP migration traps: pre-existing un-cloned mutation, and MCP cannot express clear-content'
finding: 'Two things about the MCP migration the plan did not anticipate. (1) internal/mcp/tools_entity.go:200 does `e, getErr := st.GetEntity(ctx, id)` then mutates e.Properties IN PLACE at :222-228 with no Clone(). This is safe today only by backend accident — memstore clones on read (memstore.go:194) and fsstore re-parses from disk (fsstore/entity.go:28) — but it is not guaranteed by the store contract. Migrating to PatchEntity removes the hazard by construction since PatchEntity owns the read and the clone; this is a genuine benefit of the migration worth stating rather than leaving as an accident that happens to work. (2) MCP currently CANNOT express ''clear content'': the guard at tools_entity.go:209 short-circuits when len(properties)==0 && content=="", and content is only assigned when non-empty (:227-229). The migration must not silently change that — EntityPatch.Content is a *string tri-state, so a naive mapping of MCP''s empty content to a non-nil pointer would newly allow (or newly force) body clearing. Decide deliberately and pin whichever semantics is chosen.'
severity: minor
resolution: |-
    RESOLVED both parts.

    (1) Un-cloned mutation at tools_entity.go:200/:222-228 is fixed by construction — PatchEntity owns the read and the clone. Note it in the commit message as a fixed latent hazard rather than an incidental refactor.

    (2) Clear-content: PRESERVE current behaviour. Map MCP's empty content to Content: nil (leave untouched), never to a pointer-to-empty-string. Chosen as the lowest-risk option: MCP is slated for replacement by a remote-oriented server, so spending a wire-shape decision on the current client-side implementation is not worthwhile. Keeps AC6's 'behave unchanged' promise strictly intact. Pin with a regression test — AC6 covers MCP nil-deletes but not this adjacent case.
status: addressed
---

## Resolution

1. **Un-cloned mutation** — no action needed beyond the migration itself, but
note it in the commit message as a fixed latent hazard rather than an incidental
refactor. Evidence: `tools_entity.go:200` then `:222-228`;
`memstore/memstore.go:194` returns `e.Clone()`; `fsstore/entity.go:28`
re-parses.

2. **Clear-content** — preserve today's behaviour by mapping MCP's empty content
to `Content: nil` (leave untouched), NOT to a pointer-to-empty-string. If
exposing clear-content to MCP is desired, that is a deliberate capability
addition needing its own wire-shape decision (MCP uses in-band `nil` for
properties; content has no equivalent sentinel today). Pin the chosen behaviour
with a regression test either way — AC6 already requires MCP nil-deletes to
behave unchanged, and this is the adjacent case it does not currently cover.
