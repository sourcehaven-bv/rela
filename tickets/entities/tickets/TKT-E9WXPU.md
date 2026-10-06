---
id: TKT-E9WXPU
type: ticket
title: Guard incoming relation lists in autosave with version tokens
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

TKT-2VDVHF guards autosave with per-field version tokens. The `relations` token
covers outgoing edges only. A `direction: incoming` relation widget PATCHes the
full incoming list under the inverse key, which replaces it without any
precondition. A concurrent link from another user to the same entity is then
deleted silently (RR-N8UUS1). This predates TKT-2VDVHF.

## Approach

- Serve a token per inverse key (or one incoming-relations token) that covers
the visible incoming edges, and check it in the PATCH when the request writes
that key.
- The relation picker loads incoming lists itself, so the token must reach
it with the list it loads, not only with the entity GET.
- Measure the added relation query against the entity GET budget before
putting it on every GET; serving it with the picker's load may be enough.
- Extend `mergeRelations` so incoming keys merge as sets like outgoing ones.
