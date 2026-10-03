---
id: DEC-OHJEKG
type: decision
title: Co-edit entity bodies with Yjs via a Go relay; clients save markdown
context: Two users editing one body get silent last-write-wins; Milkdown's collab plugin needs a server and rela has no Node runtime and markdown as its source of truth.
consequences: Saves keep going through PATCH so ACL/validation/audit/versioning are unchanged; saving depends on a connected browser; ygo becomes a dependency; multi-node postgres needs sticky routing until a cross-node relay exists.
date: "2026-09-25"
status: accepted
---

## Decision

Real-time multi-user editing of entity bodies uses Yjs through
`@milkdown/plugin-collab`, with a Go WebSocket relay per entity and client-side
markdown saves through the normal PATCH path (RES-L4FVT0, Option A).

A server-authoritative variant (Go writes the markdown) stays possible later: it
uses the same browser stack and sync protocol, and changes only who saves.
