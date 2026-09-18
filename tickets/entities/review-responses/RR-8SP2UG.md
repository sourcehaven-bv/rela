---
id: RR-8SP2UG
type: review-response
title: Pre-link fails open to silence when the peer id prefix does not resolve
finding: DynamicForm.vue:1417-1421 resolves the peer's type client-side via getTypeFromId(peer) and, when the prefix does not resolve, skips the relation creation with no else branch — producing a created but unlinked entity and no error on the navigate path. AC5 makes the pre-link load-bearing ("the user never links manually"), yet the only thing enforcing it is a client-side prefix match that fails open. This is the second instance of the same silent-skip class in this codebase; the folded-in SidePanel param mismatch is the first.
severity: significant
resolution: 'Added to scope as security item 5 and as AC11: an unresolvable link_peer must surface a visible failure rather than a skipped step, with a negative test for an unknown-prefix peer. Shipping buttons that depend on the pre-link while it fails silently would multiply orphans rather than prevent them, which inverts the ticket''s purpose.'
status: addressed
---

## Finding

```js
const peerType = getTypeFromId(peer)
if (peerType) {
  await createRelation(peerType, peer, relation, entity.id)
}
```

`DynamicForm.vue:1417-1421`. There is no `else`. A `link_peer` whose id prefix
the client-side schema lookup does not recognise produces a created,
**unlinked** entity with no error at all on the navigate path.

AC5 promises "the created entity is linked to the originator, with no manual
linking step". The only thing enforcing that promise is a client-side prefix
match that fails open to silence.

This is the second instance of the same class in this codebase — the
`SidePanel.vue` param mismatch folded into this ticket is the first. Both share
a shape: a link step that is skipped rather than failed.

## Resolution

Brought into scope. The plan now carries it as Security Considerations item 5
and as AC11: an unresolvable `link_peer` must surface a visible failure, never a
silently unlinked entity, with a negative test for an unknown-prefix peer.

The reasoning for including it rather than deferring: this ticket adds buttons
whose entire value is that the relation is set automatically. Shipping them on
top of a pre-link that fails silently would multiply orphans rather than prevent
them, which inverts the ticket's purpose.
