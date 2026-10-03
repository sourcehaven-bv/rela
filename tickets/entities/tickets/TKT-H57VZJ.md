---
id: TKT-H57VZJ
type: ticket
title: 'View traversal: a denied world prime falls through to a readable face'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

TKT-7IZHP0 PR 3 made every bare-id read resolve a neighbour as "the ACL trims
its faces, then the world ranks the rest": the single GET, lists, `?include=`,
response links and search. View traversal (`internal/dataentry/views.go`
`readableViewIDs`, `viewworld.go` `gateLoadedEntities`) still loads the
world-preferred face first and then checks it against the per-face verdict. A
principal whose grant denies that face loses the neighbour from the view,
although the same principal can GET it at another face.

The result is narrower, never wider, so it is not a leak. It is a surface
disagreement that the other read paths no longer have.

## Approach

Resolve view hops through `visibility.Resolver.ResolveIDs` (or the view reader
equivalent) so the verdict trims candidate faces before the world ranks them.
Add a view case to `TestFaceGateParity_SurfacesAgreeUnderOneWorld`. Then remove
the sentence about views from `docs/acl-security.md`.
