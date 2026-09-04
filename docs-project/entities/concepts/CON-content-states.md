---
id: CON-content-states
type: concept
title: "Faces and Worlds (Content States)"
summary: "An entity can hold several content states, called faces; a world is a declared rule that picks one face per entity for a reader"
---

A **face** is one content state of an entity: the draft of a policy and its
published text, or the English and Dutch versions of a guide. Every face
shares the entity's id, and one of them is the **bare face**, the state a bare
id such as `POL-1` addresses.

A **world** is a named, declared rule that projects the graph down to one face
per entity. A `published` world shows readers only what has been published
and leaves everything else out. An `editorial` world prefers drafts. A
`site-nl` world prefers Dutch and falls back to English.

A world is selected per request with the `?world=` query parameter, and
reading one is a permission of its own. Writes never take a world: ordinary
writes address the bare face, and other faces are written through declared
**copy definitions**, which is how publishing becomes an authorized
operation rather than a field edit.

See the guide "How To Publish Content with Faces and Worlds" (`docs/content-states.md`)
for the full walkthrough.
