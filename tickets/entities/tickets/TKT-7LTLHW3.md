---
id: TKT-7LTLHW3
type: ticket
title: Verified fake API and property tests for SPA editing
kind: test
priority: high
effort: l
started: "2026-10-29"
completed: "2026-10-30"
status: done
---

## Description

Switching items in the SPA and then editing can write to the previous item. The
edit path reads the entity id when a save fires (`getEntityId: () =>
props.entityId` in `DynamicForm.vue`, `owner.id` in `SectionEditForm.vue`),
while the form data belongs to whichever load finished last. The outcome depends
on the order of route changes, GET responses, debounce timers, in-flight PATCHes
and SSE refetches. Example-based tests check one order.

Add model-based property tests (fast-check `fc.commands` + `fc.scheduler`) for
the SPA's editing surfaces. They run against an in-process fake of the
data-entry API. The fake is verified against the real `rela-server` on the
memory store by a differential test, so it cannot drift silently.

## Scope

1. **Fake API** (TypeScript, in-process) for the entity-editing slice:
static schema/config snapshot taken from the real server; entity get, list,
create, update, delete; per-field `_versions` tokens and 412 conflicts; SSE
change events. Out of scope: ACL, automations, validation rules, `_actions`.
Tests for those use the real server.
2. **Differential test**: random command sequences, including several
clients with stale tokens, run through `src/api` against the fake and against
`rela-server -tags memorybackend`. Responses are compared after normalizing ids,
timestamps and order. Runs in CI on changes to `internal/dataentry`,
`frontend/src/api` or the fake.
3. **First SPA property test**: entity switch then edit on the detail page
and form. Invariants: every PATCH targets the entity on screen when the edit was
made; the form shows the routed entity's data or nothing; after settling, server
state matches the model.

## Acceptance criteria

- The differential test passes and fails when the fake is changed to drop a
412 or a `_versions` token (mutation check).
- The entity-switch property test reproduces the reported bug without being
given the ordering, and passes after the fix.
- Tests are deterministic for a seed; the seed is logged on failure.
