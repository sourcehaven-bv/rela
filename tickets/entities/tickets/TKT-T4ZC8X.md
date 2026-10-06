---
id: TKT-T4ZC8X
type: ticket
title: Investigate Milkdown concurrent (multi-user) editing for entity bodies
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Investigate using Milkdown's concurrent (multi-user) editing support for entity
markdown bodies in the data-entry SPA, and propose implementation options.

Milkdown ships `@milkdown/plugin-collab`, a binding to Yjs (a CRDT library) via
y-prosemirror. rela's source of truth is markdown text per entity, the server is
Go with no Node.js runtime, and postgres deployments may run several
`rela-server` nodes against one database. Each of these constrains how a shared
editing session can be hosted and persisted.

TKT-2VDVHF (autosave conflict resolution) put CRDT editing explicitly out of
scope; this ticket is where that question is answered.

## Deliverable

A research write-up with options, trade-offs, effort and a recommendation.
Implementation follows in separate tickets once an option is chosen.
