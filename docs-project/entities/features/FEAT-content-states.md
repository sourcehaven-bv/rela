---
id: FEAT-content-states
type: feature
title: "Faces and Worlds"
status: published
summary: "Per-entity content states (draft, published, translations) and declared worlds that pick one face per reader"
---

Declare `faces:` on an entity type, `worlds:` that select among them, and
`copies:` that move content between faces under a permission guard. The web
app and the HTTP API serve a world per request; reading a world is an ACL
grant of its own.
