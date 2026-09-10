---
id: CON-content-states
type: concept
title: "Faces and Worlds (Content States)"
summary: "An entity can hold several content states, called faces; a world is a declared rule that picks one face per entity for a reader"
---

A **face** is one content state of an entity: the draft of a policy and its
published text, or the English and Dutch versions of a guide. Every face shares
the entity's id and is addressed by name after it, as `POL-1@draft`. A type
that declares no faces has a single state addressed by the bare id.

A **world** is a named, declared rule that projects the graph down to one face
per entity. A `published` world shows readers only what has been published
and leaves everything else out. An `editorial` world prefers drafts. A
`site-nl` world prefers Dutch and falls back to English.

A world is selected per request with the `?world=` query parameter, and
reading one is a permission of its own. Writes never take a world: a write
names the face it changes, and moving content between faces happens through
declared **copy definitions**, which is how publishing becomes an authorized
operation rather than a field edit.

See the guide "How To Publish Content with Faces and Worlds" (`docs/content-states.md`)
for the full walkthrough.
