---
id: RR-3VSSPM
type: review-response
title: Comment IDs, ordering and the update/delete authorisation subject are undefined
finding: 'The record lists an ''ID'' field but the plan never says who mints it, what shape it takes, or whether it is unique globally or only per target. Three consequences left to the implementer: (1) If the client supplies the ID, a caller can overwrite another user''s comment by guessing/reusing an ID — the server must mint it, consistent with the author-stamping rationale. (2) Ordering is unspecified; a comment thread with no defined order will render differently across backends, and commentstest.RunAll cannot assert what the interface does not promise. CreatedAt exists but ties are undefined. (3) Most importantly, comment:update and comment:delete are resolved against the TARGET ENTITY, so any holder can edit or delete ANY comment on that entity, including other people''s. That may be intended for a moderator role, but the plan never states it, and it is the natural place a reader would expect ''edit your own'' — which RES-XRYX18 explicitly defers as needing new ACL machinery. State the intended semantics; if ''own comments only'' is wanted, it needs its own ticket, and if not, say so in the docs so operators are not surprised.'
severity: significant
resolution: |-
    Resolved by splitting the mutating permissions into explicit own/any pairs rather than reusing the graph's ownership mechanism.

    Why not the graph's 'own': there is no `own` primitive in rela's ACL. Ownership is a GRAPH EDGE — `role_relations: {assigned-to: {confers: assignee}}` — and computeForEntity (resolver.go:176) tests `r.d.graph.HasEdge(member, relType, target)`. Since comments are deliberately not in the graph, HasEdge has nothing to test, so reusing that machinery would require teaching the ACL graph walker about a non-graph store — exactly the coupling that motivated separating comments in the first place.

    Final vocabulary (6 permissions, naming follows the history:read-redacted convention):
      comment:read        — see comments on this entity
      comment:add         — add a comment
      comment:update-own  — edit/resolve a comment you authored
      comment:update-any  — edit/resolve anyone's comment (moderator)
      comment:delete-own  — delete a comment you authored
      comment:delete-any  — delete anyone's comment (moderator)

    'Own' is decided by comparing the stored Author against principal.From(ctx) — a string comparison inside the comment service, needing no ACL change. -any implies -own (holding -any satisfies an -own check).

    Also resolved in the same change: the server mints the comment ID (never the client, same rationale as author stamping), and List returns comments ordered by CreatedAt ascending with the minted ID as the deterministic tie-break, pinned by commentstest.RunAll so all backends agree.
status: addressed
---
