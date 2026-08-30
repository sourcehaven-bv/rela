---
id: TKT-IVSJV6
type: ticket
title: Replace producer-side entitymanager.EntityManager with per-consumer interfaces
kind: refactor
priority: medium
effort: m
status: done
---

## Description

`entitymanager.EntityManager` is a wide producer-side interface declared
alongside its sole implementation (`Manager`) — against CLAUDE.md's "define
interfaces at the call site" rule. Its own godoc says so and marks itself
transitional and slated for removal. GitHub issue #741.

The interface persisted because the removed workspace bridge returned it from
`Workspace.Manager()`. Every consumer that touches it today pulls in the full
10-method surface, leaking unused methods into test fixtures and arch-lint
footprints.

Each consumer should declare its own narrow interface naming only the methods it
invokes; `*Manager` continues to satisfy each structurally. The interface is
then deleted and producer code uses `*Manager` directly.

No behaviour change.
